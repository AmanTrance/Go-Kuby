package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/valkey-io/valkey-go"
)

var maxGoRoutineCount int = 4
var sleepTime int64 = 10
var goRoutineCount int = 2
var processInQueue int = 0

type RequestObject struct {
	Endpoint string              `json:"endpoint"`
	Body     any                 `json:"body,omitempty"`
	Params   map[string][]string `json:"params"`
	Method   string              `json:"method"`
}

type SleepBody struct {
	SecretKey string `json:"secretKey"`
	Value     int64  `json:"value"`
}

type ThreadBody struct {
	SecretKey string `json:"secretKey"`
	Value     int    `json:"value"`
}

func main() {
	valkeyClient, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{"172.21.128.1:6379"}})
	if err != nil {
		fmt.Printf("error\n")
		fmt.Printf("%s\n", err.Error())
	}
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mutex sync.Mutex
	counterChannel := make(chan int)
	defer close(counterChannel)

	go func(ctx context.Context, client valkey.Client, threadCount *int, m *sync.Mutex) {
		command := client.B().Subscribe().Channel("AsvatthiChannel").Build()
		err := client.Receive(ctx, command, func(msg valkey.PubSubMessage) {
			processInQueue--
			message := msg.Message
			var object RequestObject
			err := json.Unmarshal([]byte(message), &object)
			if err != nil {
				fmt.Printf("error\n")
				fmt.Printf("%s\n", err.Error())
			}
			(*m).Lock()
			(*threadCount)++
			go HandleComputeHeavyFunc(object, threadCount, m)
			(*m).Unlock()
			PowerfulWait(threadCount)
		})
		if err != nil {
			fmt.Printf("error\n")
			fmt.Printf("%s\n", err.Error())
		}
		fmt.Printf("Receiver finished")
		(*m).Lock()
		(*threadCount)++
		(*m).Unlock()
	}(ctx, valkeyClient, &goRoutineCount, &mutex)

	http.HandleFunc("/sleep", HandleSleepTime)
	http.HandleFunc("/thread", HandleThreadCount)
	http.HandleFunc("/count", GetCurrentThreadCount)
	http.HandleFunc("/path", AddContextMetaData(valkeyClient, &mutex))
	http.ListenAndServe(":8000", nil)
}

func AddContextMetaData(client valkey.Client, m *sync.Mutex) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var body any
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			fmt.Printf("decodeError\n")
			fmt.Printf("%s\n", err.Error())
		}
		var params map[string][]string = map[string][]string{}
		for k, v := range r.URL.Query() {
			params[k] = v
		}
		valkeyRequestObject := RequestObject{
			Endpoint: r.URL.Path,
			Body:     body,
			Params:   params,
			Method:   r.Method,
		}
		data, err := json.Marshal(&valkeyRequestObject)
		if err != nil {
			fmt.Printf("error\n")
			fmt.Printf("%s\n", err.Error())
		}
		command := client.B().Publish().Channel("AsvatthiChannel").Message(string(data)).Build()
		err = client.Do(r.Context(), command).Error()
		if err != nil {
			fmt.Printf("error\n")
			fmt.Printf("%s\n", err.Error())
		}
		(*m).Lock()
		processInQueue++
		(*m).Unlock()
	}
}

func HandleComputeHeavyFunc(r RequestObject, threadCount *int, m *sync.Mutex) {
	time.Sleep(time.Second * 20)
	fmt.Printf("%v\n", r)
	(*m).Lock()
	(*threadCount)--
	(*m).Unlock()
}

func PowerfulWait(count *int) {
	if *count == maxGoRoutineCount {
		time.Sleep(time.Second * time.Duration(sleepTime))
		PowerfulWait(count)
	}
	return
}

func HandleSleepTime(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body SleepBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	if body.SecretKey != "binary" {
		w.WriteHeader(400)
		return
	}
	sleepTime = body.Value
	w.WriteHeader(200)
	return
}

func HandleThreadCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body ThreadBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	if body.SecretKey != "binary" {
		w.WriteHeader(400)
		return
	}
	if body.Value < 3 {
		w.WriteHeader(400)
		return
	}
	maxGoRoutineCount = body.Value
	w.WriteHeader(200)
	return
}

func GetCurrentThreadCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var response struct {
		CurrentCount   int `json:"current_count"`
		MaxCount       int `json:"max_count"`
		ProcessInQueue int `json:"process_in_queue"`
	}
	response.CurrentCount = goRoutineCount
	response.MaxCount = maxGoRoutineCount
	response.ProcessInQueue = processInQueue
	JSONResponse, err := json.Marshal(&response)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(JSONResponse)
	return
}
