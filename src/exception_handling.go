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
	if errorMessage == nil {
		return
	}

	// Create formatted error message and give to message func
	logMessage("Error: %s: %v", errorDescription, errorMessage)

	// Exit prog after sending error messages
	if exitRequested {
		os.Exit(1)
	}
}

// Send message string to remote log server or stdout if remote log not enabled
func logMessage(message string, vars ...any) {
	var err error

	// Write to remote socket
	if remoteLogEnabled {
		err = logToRemote(message, vars...)
		// Prep err from functions for writing to stdout
		if err != nil && err.Error() != "syslogAddress is empty" {
			message = "Failed to send message to desired location: " + err.Error() + " - ORIGINAL MESSAGE: " + message
		}
	}

	// Newlines for stdout
	message = message + "\n"

	fmt.Printf(message, vars...)
}

// Sends message to remote syslog server in standard-ish format
func logToRemote(message string, vars ...any) error {
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

	logText := fmt.Sprintf(message, vars...)

	// Format message
	syslogMsg := fmt.Sprintf("<%d>%s %s: %s", syslog.LOG_INFO, timestamp, "wol-server", logText)

	// Write message to remote host - failure writes to stdout
	_, err = conn.Write([]byte(syslogMsg))
	if err != nil {
		return err
	}

	return nil
}
