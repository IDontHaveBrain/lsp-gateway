package workspace

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Example demonstrates basic usage of WorkspacePortManager
func ExampleWorkspacePortManager_basic() {
	// Create a new port manager
	pm, err := NewWorkspacePortManager()
	if err != nil {
		log.Fatal(err)
	}

	// Create a temporary workspace directory
	workspaceRoot := filepath.Join(os.TempDir(), "example-workspace")
	os.MkdirAll(workspaceRoot, 0755)
	defer os.RemoveAll(workspaceRoot)

	// Allocate a port for the workspace
	port, err := pm.AllocatePort(workspaceRoot)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Allocated port %d for workspace %s\n", port, workspaceRoot)

	// Check if port is assigned
	assignedPort, exists := pm.GetAssignedPort(workspaceRoot)
	if exists {
		fmt.Printf("Workspace has assigned port: %d\n", assignedPort)
	}

	// Release the port when done
	if err := pm.ReleasePort(workspaceRoot); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Port released successfully")
}

// Example demonstrates consistent port assignment based on workspace hash
func ExampleWorkspacePortManager_consistency() {
	pm, err := NewWorkspacePortManager()
	if err != nil {
		log.Fatal(err)
	}

	workspaceRoot := "/home/user/my-project"

	// First allocation
	port1, err := pm.AllocatePort(workspaceRoot)
	if err != nil {
		log.Fatal(err)
	}

	// Release the port
	pm.ReleasePort(workspaceRoot)

	// Second allocation should return the same port (if available)
	port2, err := pm.AllocatePort(workspaceRoot)
	if err != nil {
		log.Fatal(err)
	}

	if port1 == port2 {
		fmt.Printf("Consistent port assignment: %d\n", port1)
	}

	pm.ReleasePort(workspaceRoot)
}

// Example demonstrates handling multiple workspaces
func ExampleWorkspacePortManager_multipleWorkspaces() {
	pm, err := NewWorkspacePortManager()
	if err != nil {
		log.Fatal(err)
	}

	workspaces := []string{
		"/home/user/project-a",
		"/home/user/project-b", 
		"/home/user/project-c",
	}

	allocatedPorts := make(map[string]int)

	// Allocate ports for multiple workspaces
	for _, workspace := range workspaces {
		port, err := pm.AllocatePort(workspace)
		if err != nil {
			log.Fatal(err)
		}
		allocatedPorts[workspace] = port
		fmt.Printf("Workspace %s -> Port %d\n", filepath.Base(workspace), port)
	}

	// Verify all ports are different
	portSet := make(map[int]bool)
	for _, port := range allocatedPorts {
		if portSet[port] {
			fmt.Println("Error: Duplicate port allocation detected!")
		}
		portSet[port] = true
	}

	// Release all ports
	for _, workspace := range workspaces {
		pm.ReleasePort(workspace)
	}

	fmt.Printf("Successfully managed %d workspaces\n", len(workspaces))
}

// Example demonstrates error handling
func ExampleWorkspacePortManager_errorHandling() {
	pm, err := NewWorkspacePortManager()
	if err != nil {
		log.Fatal(err)
	}

	// Try to allocate port for invalid workspace
	invalidWorkspace := "/nonexistent/invalid/path"
	_, err = pm.AllocatePort(invalidWorkspace)
	
	if err != nil {
		// Handle different types of port errors
		if portErr, ok := err.(*PortError); ok {
			switch portErr.Type {
			case PortErrorTypeAllocation:
				fmt.Println("Port allocation failed")
				fmt.Printf("Suggestions: %v\n", portErr.Suggestions)
			case PortErrorTypeAvailability:
				fmt.Printf("Port %d is not available\n", portErr.Port)
			case PortErrorTypeLocking:
				fmt.Printf("Could not lock port %d\n", portErr.Port)
			default:
				fmt.Printf("Port error: %s\n", portErr.Type)
			}
		} else {
			fmt.Printf("General error: %v\n", err)
		}
	}
}

// Example demonstrates checking port availability
func ExampleWorkspacePortManager_portAvailability() {
	pm, err := NewWorkspacePortManager()
	if err != nil {
		log.Fatal(err)
	}

	// Check various ports for availability
	testPorts := []int{8080, 8081, 8082, 9999, 80}
	
	fmt.Println("Port availability check:")
	for _, port := range testPorts {
		available := pm.IsPortAvailable(port)
		fmt.Printf("Port %d: %t\n", port, available)
	}
}

// Example demonstrates workspace port file creation
func ExampleWorkspacePortManager_portFiles() {
	pm, err := NewWorkspacePortManager()
	if err != nil {
		log.Fatal(err)
	}

	// Create temporary workspace
	workspaceRoot := filepath.Join(os.TempDir(), "port-file-example")
	os.MkdirAll(workspaceRoot, 0755)
	defer os.RemoveAll(workspaceRoot)

	// Allocate port
	port, err := pm.AllocatePort(workspaceRoot)
	if err != nil {
		log.Fatal(err)
	}

	// Check if port file was created
	portFilePath := filepath.Join(workspaceRoot, PortFileName)
	if _, err := os.Stat(portFilePath); err == nil {
		fmt.Printf("Port file created at: %s (port %d)\n", portFilePath, port)
		
		// Read port file content
		data, err := os.ReadFile(portFilePath)
		if err == nil {
			fmt.Printf("Port file content:\n%s\n", string(data))
		}
	}

	// Release port (removes port file)
	pm.ReleasePort(workspaceRoot)

	// Verify port file is removed
	if _, err := os.Stat(portFilePath); os.IsNotExist(err) {
		fmt.Println("Port file removed after release")
	}
}