package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// Client is used to talk to the pom server
type Client struct {
	host string
	port string
	conn *http.Client // conn isn't a great name, but I can't think of a better one
}

// New returns a new pom client
func New() *Client {
	httpClient := &http.Client{}
	client := Client{
		conn: httpClient,
		host: "localhost",
		port: "13381",
	}
	return &client
}

// do executes a http request
func (client Client) do(url string) {
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		log.Printf("Error creating pom request: %s", err.Error())
		return
	}

	resp, err := client.conn.Do(req)
	if err != nil {
		log.Printf("Error reading body: %s", err.Error())
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading body: %s", err.Error())
		return
	}
	defer resp.Body.Close()

	fmt.Printf("%s\n", body)
}

// Start will start a pom
func (client Client) Start(name string) {
	client.do(fmt.Sprintf("http://localhost:13381/start/%s", name))
}

// Cancel will cancel a pom
func (client Client) Cancel() {
	client.do("http://localhost:13381/cancel")
}

// Pause will pause a pom
func (client Client) Pause() {
	client.do("http://localhost:13381/pause")
}

// Resume will resume a pom
func (client Client) Resume() {
	client.do("http://localhost:13381/resume")
}

// Status will return the status of your poms
func (client Client) Status() {
	client.do("http://localhost:13381/status")
}

// Clear will clear your poms
func (client Client) Clear() {
	client.do("http://localhost:13381/clear")
}
