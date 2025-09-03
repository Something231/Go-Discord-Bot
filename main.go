// Token should be stored in token.txt
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const useragent string = "DiscordBot (https://github.com/Something231/Go-Discord-Bot, 0.0.1)"

// var lastsequencenumber int
var heartbeatrecieved int32
var sequencenumber int64

var session_id string
var resume_gateway_url string
var guilds []string
var apiversion int

var token string

var gateway struct {
	URL string `json:"url"`
}

var lastMessageId string

var wg sync.WaitGroup

var stopChan = make(chan struct{})
var resume_procedure uint8 //1: Restart connection completely, 2: resume connection with url.

func terminate(i uint8, connection *websocket.Conn) {
	close(stopChan)
	if i == 0 {
		fmt.Println("OHDEAR I = 0")
	}
	resume_procedure = i
	wg.Wait()
	connection.Close()
	fmt.Println("Connection was terminated but like in  a good way")
	fmt.Println("Goroutines running:", runtime.NumGoroutine())
}

func get_error(err error) uint8 {
	if closeErr, ok := err.(*websocket.CloseError); ok {
		fmt.Printf("Recieved close code: %v", closeErr.Code)
		switch closeErr.Code {
		case 4000, 4001, 4002, 4003, 4004, 4005, 4007, 4008, 4009:
			return 2
		default:
			fmt.Println("Unknown CloseErr")
			return 1
		}
	} else {
		fmt.Println("Unknown websocket error", err)
		return 0
	}
}

func get_gateway() {
	response, err := http.Get("https://discord.com/api/gateway")
	if err != nil {
		fmt.Println("Request to discord servers failed.")
		os.Exit(1)
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)

	if response.StatusCode > 299 {
		fmt.Printf("Response failed with status code: %d and\nbody: %s\n", response.StatusCode, body)
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = json.Unmarshal(body, &gateway)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func open_connection(url string) (int, *websocket.Conn) {
	connection, _, err := websocket.DefaultDialer.Dial(url+"/?v=10&encoding=json", nil)

	if err != nil {
		fmt.Println("Failed to connect to gateway :(")
		os.Exit(1)
	}

	fmt.Println("Connected to discord gateway")

	var hello struct {
		OP int `json:"op"`
		D  struct {
			HeartbeatInterval int `json:"heartbeat_interval"`
		} `json:"d"`
	}

	for {
		_, message, err := connection.ReadMessage()
		if err != nil {
			fmt.Println("read:", err)
			os.Exit(1)
		}
		err = json.Unmarshal(message, &hello)
		if err != nil {
			fmt.Println("Something has gone bad bad very bad", err)
		} else if hello.OP == 10 {
			fmt.Println("Hello event processed")
			break
		}
	}

	return hello.D.HeartbeatInterval, connection
}

func resume_event(connection *websocket.Conn) bool {
	var res_payload struct {
		OP int `json:"op"`
		D  struct {
			TOKEN  string `json:"token"`
			SESSID string `json:"session_id"`
			SEQ    int    `json:"seq"`
		} `json:"d"`
	}
	res_payload.OP = 6
	res_payload.D.TOKEN = token
	res_payload.D.SESSID = session_id
	res_payload.D.SEQ = int(sequencenumber)
	err := connection.WriteJSON(res_payload)
	if err != nil {
		return false
	}
	return true
}

func heartbeat(heartbeat_interval int, connection *websocket.Conn) {
	ticker := time.NewTicker(time.Duration(float64(heartbeat_interval)*rand.Float64()) * time.Millisecond)
	defer ticker.Stop()

	select {
	case <-stopChan:
		fmt.Println("Stopped INSTANTLY :0")
		return
	case <-ticker.C:
		fmt.Println("initial heartbeat")
		heartbeaterr := connection.WriteMessage(websocket.TextMessage, []byte(`{"op":1,"d":null}`))

		if heartbeaterr != nil {
			fmt.Println("I don't know what this error is:", heartbeaterr)
			return
		}
	}

	ticker = time.NewTicker(time.Duration(float64(heartbeat_interval)) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			if atomic.LoadInt32(&heartbeatrecieved) == 0 {
				println("At this point, this is where you know its over.")
				terminate(2, connection)
				return
			}
			var heartbeaterr error
			fmt.Println("sending heartbeat")
			if atomic.LoadInt64(&sequencenumber) == 0 {
				heartbeaterr = connection.WriteMessage(websocket.TextMessage, []byte(`{"op":1,"d":null}`))
			} else {
				heartbeaterr = connection.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"op":1,"d":%d}`, atomic.LoadInt64(&sequencenumber)))
			}
			if heartbeaterr != nil {
				terminate(get_error(heartbeaterr), connection)
				return
			}
			atomic.StoreInt32(&heartbeatrecieved, 0)
		}
	}
}

func event_handler(connection *websocket.Conn) {
	for {
		print(1)
		var event struct {
			OP int `json:"op"`
			D  struct {
				APIVERSION int    `json:"v"`
				SESSIONID  string `json:"session_id"`
				RESUMEURL  string `json:"resume_gateway_url"`
				USER       struct {
					USERNAME      string `json:"username"`
					DISCRIMINATOR string `json:"discriminator"`
				} `json:"user"`
				GUILDS []struct {
					ID        string `json:"id"`
					UNAGUILDS bool   `json:"guilds"`
				} `json:"guilds"`
				CONTENT   string `json:"content"`
				CHANNELID string `json:"channel_id"`
				ID        string `json:"id"`
			} `json:"d"`
			S int    `json:"s"`
			T string `json:"t"`
		}
		readerr := connection.ReadJSON(&event)
		if readerr != nil {
			terminate(get_error(readerr), connection)
			return
		}
		switch event.OP {
		case 11:
			atomic.StoreInt32(&heartbeatrecieved, 1)
			fmt.Println("heartbeat recieved")
		case 1:
			fmt.Println("Spanish Inquisition apparently")
			var heartbeaterr error
			if atomic.LoadInt64(&sequencenumber) == 0 {
				heartbeaterr = connection.WriteMessage(websocket.TextMessage, []byte(`{"op":1,"d":null}`))
			} else {
				heartbeaterr = connection.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"op":1,"d":%d}`, atomic.LoadInt64(&sequencenumber)))
			}
			if heartbeaterr != nil {
				terminate(get_error(readerr), connection)
				return
			}
		case 0:
			fmt.Println(event.T)
			switch event.T {
			case "READY":
				apiversion = event.D.APIVERSION
				session_id = event.D.SESSIONID
				resume_gateway_url = event.D.RESUMEURL
				for _, guild := range event.D.GUILDS {
					fmt.Println(guild.ID)
					guilds = append(guilds, guild.ID)
				}
				fmt.Printf("Signed in as %v#%v\n", event.D.USER.USERNAME, event.D.USER.DISCRIMINATOR)
			case "MESSAGE_CREATE":
				fmt.Println(event.D.CONTENT)
				if event.D.ID == lastMessageId {
					fmt.Println("Still resuming i guess...")
				} else if event.D.CONTENT == "e" {
					var message struct {
						CONTENT string `json:"content"`
					}
					message.CONTENT = "UwU"
					jsonmessage, jsonerr := json.Marshal(message)
					if jsonerr != nil {
						fmt.Println("Failed to convert message to json.")
					}
					request, httperr := http.NewRequest("POST", ("https://discord.com/api/channels/" + event.D.CHANNELID + "/messages"), strings.NewReader(string(jsonmessage)))
					if httperr != nil {
						fmt.Println("Http Error")
					}
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set("Authorization", ("Bot " + token))
					request.Header.Set("User-Agent", useragent)
					response, httperr := http.DefaultClient.Do(request)
					if httperr != nil {
						fmt.Println("Failed to send message")
					}
					if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
						fmt.Printf("Failed to send message: %s\n", response.Status)
					} else {
						fmt.Println("Message sent successfully!")
					}
					lastMessageId = event.D.ID
					response.Body.Close()
					terminate(2, connection)
					return
				}
				lastMessageId = event.D.ID
			}
		case 9:
			fmt.Println("Invalid Session Error Has occured -- Re-Identifying")
			//not even gonna bother with the potential for reconnection
			terminate(1, connection)
			return
		case 7:
			terminate(2, connection)
			return
		}
		fmt.Println("Event Confirmed; OP CODE:", event.OP)
		if event.S != 0 {
			atomic.StoreInt64(&sequencenumber, int64(event.S))
			fmt.Println("Sequence Updated", sequencenumber)
		}
	}
}

func identify(connection *websocket.Conn) {
	var identifypayload struct {
		OP int `json:"op"`
		D  struct {
			TOKEN      string `json:"token"`
			INTENTS    int    `json:"intents"`
			PROPERTIES struct {
				OS      string `json:"os"`
				BROWSER string `json:"browser"`
				DEVICE  string `json:"device"`
			} `json:"properties"`
		} `json:"d"`
	}

	identifypayload.OP = 2
	identifypayload.D.INTENTS = 131071
	identifypayload.D.PROPERTIES.OS = "linux"
	identifypayload.D.PROPERTIES.BROWSER = "my_library"
	identifypayload.D.PROPERTIES.DEVICE = "my_library"
	identifypayload.D.TOKEN = token
	err := connection.WriteJSON(identifypayload)
	if err != nil {
		fmt.Println("its so over - quitting since if the identify fails then its all joever")
		os.Exit(1)
	}

}

func main() {
	defer fmt.Println("Program exited")
	get_gateway()
	heartbeat_interval, connection := open_connection(gateway.URL)

	wg.Add(1)
	go func() {
		defer wg.Done()
		heartbeat(heartbeat_interval, connection)
	}()

	go event_handler(connection)

	tokendata, err := os.ReadFile("token.txt")
	if err != nil {
		fmt.Println("Error reading token file. Set token in token.txt.")
		os.Exit(1)
	}

	token = string(tokendata)

	identify(connection)
	fmt.Println("Goroutines running:", runtime.NumGoroutine())
	defer connection.Close()
	for {
		print(runtime.NumGoroutine())
		<-stopChan
		print("Connection was terminated oh deary me", resume_procedure)
		stopChan = make(chan struct{})

		time.Sleep(time.Duration(2) * time.Millisecond)
		wg.Wait()
		print(2)
		switch resume_procedure {
		case 0:
			fmt.Println("An unknown error has occured")
			os.Exit(1)
		case 1:
			sequencenumber = 0
			heartbeat_interval, connection = open_connection(gateway.URL)
			wg.Add(1)
			go func() {
				defer wg.Done()
				heartbeat(heartbeat_interval, connection)
			}()
			go event_handler(connection)
			identify(connection)
		case 2:
			fmt.Println("Attempting Resuming of connection")
			heartbeat_interval, connection = open_connection(resume_gateway_url)
			resumed := resume_event(connection)
			if resumed == false {
				heartbeat_interval, connection = open_connection(gateway.URL)
				wg.Add(1)
				go func() {
					defer wg.Done()
					heartbeat(heartbeat_interval, connection)
				}()
				go event_handler(connection)
				identify(connection)
			} else {
				wg.Add(1)
				go func() {
					defer wg.Done()
					heartbeat(heartbeat_interval, connection)
				}()
				go event_handler(connection)
			}

		}
	}
}
