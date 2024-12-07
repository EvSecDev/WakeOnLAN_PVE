// wakeonlanpve
package main

import (
	"fmt"
	"log/syslog"
	"net"
	"os"
	"time"
)

// ###################################
//      EXCEPTION HANDLING
// ###################################

// Logs error description and error - will exit entire program if requested
func logError(errorDescription string, errorMessage error, exitRequested bool) {
	// Create formatted error message and give to message func
	fullMessage := "Error: " + errorDescription + ": " + errorMessage.Error()
	logMessage(fullMessage)

	// Exit prog after sending error messages
	if exitRequested {
		os.Exit(1)
	}
}

// Send message string to remote log server or stdout if remote log not enabled
func logMessage(message string) {
	var err error

	// Write to remote socket
	if remoteLogEnabled {
		err = logToRemote(message)
		if err == nil {
			return
		}
	}

	// Prep err from functions for writing to stdout
	if err != nil && err.Error() != "syslogAddress is empty" {
		message = "Failed to send message to desired location: " + err.Error() + " - ORIGINAL MESSAGE: " + message
	}

	// Write to stdout if other messages aren't selected or fail
	fmt.Printf("%s\n", message)
}

// Sends message to remote syslog server in standard-ish format
func logToRemote(message string) error {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// If no address, go to stdout write
	if syslogAddress == nil {
		return fmt.Errorf("syslogAddress is empty")
	}

	// Open socket to remote syslog server
	conn, err := net.DialUDP("udp", nil, syslogAddress)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Format message
	syslogMsg := fmt.Sprintf("<%d>%s %s: %s", syslog.LOG_INFO, timestamp, "wol-server", message)

	// Write message to remote host - failure writes to stdout
	_, err = conn.Write([]byte(syslogMsg))
	if err != nil {
		return err
	}

	return nil
}
