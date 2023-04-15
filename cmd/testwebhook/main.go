package main

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	checkToken(req, "123")
	res, err := io.ReadAll(req.Body)
	if err != nil {
		fmt.Printf("notification read error: %v", err)
		return
	}
	_ = req.Body.Close()
	fmt.Printf("Notification: %s\n", res)
	resp.WriteHeader(http.StatusOK)
}

func checkToken(req *http.Request, token string) {
	header := req.Header.Get("authorization")
	if header == "" {
		fmt.Printf("no authorization header\n")
		return
	}
	auth := strings.Split(header, " ")
	if len(auth) != 2 || auth[0] != "Bearer" {
		fmt.Printf("not Bearer token\n")
		return
	}
	if x := subtle.ConstantTimeCompare([]byte(auth[1]), []byte(token)); x == 1 {
		return
	} // constant time comparison to prevent time attack
	fmt.Printf("invalid token\n")
	return
}
