package main

import (
	"fmt"
	"io"
	"net/http"
)

func main() {
	http.HandleFunc("/webhook", getNotification)
	fmt.Printf("webhook listener started\n")
	err := http.ListenAndServe(":3333", nil)
	if err != nil {
		panic(err)
	}
}

func getNotification(resp http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		fmt.Printf("Not a post request!\n")
	}
	res, err := io.ReadAll(req.Body)
	if err != nil {
		fmt.Printf("notification read error: %v", err)
		return
	}
	_ = req.Body.Close()
	fmt.Printf("Notification: %s\n", res)
	resp.WriteHeader(http.StatusOK)
}
