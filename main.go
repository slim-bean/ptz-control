package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/icholy/digest"
	"github.com/splace/joysticks"
	"gopkg.in/yaml.v3"
)

const (
	deadzone = 0.1
)

type Config struct {
	User string `yaml:"user"`
	Pass string `yaml:"pass"`
	URL  string `yaml:"url"`
}

func main() {
	// catch signals and terminate the app
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	// monitor for signals in the background
	go func() {
		s := <-sigc
		//ticker.Stop()
		fmt.Println("\nreceived signal:", s)
		os.Exit(0)
	}()

	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Printf("Expecting a file named config.yaml with contents:\nuser: [username]\npass: [password]\nInstead received an error: %v \n", err)
		os.Exit(1)

	}
	conf := &Config{}
	err = yaml.Unmarshal(yamlFile, conf)
	if err != nil {
		log.Printf("Failed to unmarshal config.yaml, should be in the form\nuser: [username]\npass: [password]\nError: %v", err)
		os.Exit(1)
	}

	client := &http.Client{
		Transport: &digest.Transport{
			Username: conf.User,
			Password: conf.Pass,
		},
	}

	evts := joysticks.Capture(
		joysticks.Channel{1, joysticks.HID.OnClose}, // evts[0] set to receive button #1 closes events
		joysticks.Channel{1, joysticks.HID.OnOpen},
		joysticks.Channel{2, joysticks.HID.OnClose}, // evts[0] set to receive button #1 closes events
		joysticks.Channel{2, joysticks.HID.OnOpen},
		joysticks.Channel{1, joysticks.HID.OnMove},
	)

	for {
		select {
		case <-evts[0]:
			fmt.Println("Zoom Out Button Pushed")
			res, err := client.Get(conf.URL + "/cgi-bin/ptz.cgi?action=start&channel=0&code=ZoomWide&arg1=0&arg2=1&arg3=0&arg4=0")
			if err != nil {
				fmt.Printf("Error zooming in: %v\n", err)
				continue
			}
			fmt.Println("Zoom in return: ", res.StatusCode)
			res.Body.Close()
		case <-evts[1]:
			fmt.Println("Zoom Out Button Unpushed")
			res, err := client.Get(conf.URL + "/cgi-bin/ptz.cgi?action=stop&channel=0&code=ZoomWide&arg1=0&arg2=1&arg3=0&arg4=0")
			if err != nil {
				fmt.Printf("Error zooming out: %v\n", err)
				continue
			}
			fmt.Println("Zoom out return: ", res.StatusCode)
		case <-evts[2]:
			fmt.Println("Zoom In Button Pushed")
			res, err := client.Get(conf.URL + "/cgi-bin/ptz.cgi?action=start&channel=0&code=ZoomTele&arg1=0&arg2=1&arg3=0&arg4=0")
			if err != nil {
				fmt.Printf("Error zooming in: %v\n", err)
				continue
			}
			fmt.Println("Zoom in return: ", res.StatusCode)
			res.Body.Close()
		case <-evts[3]:
			fmt.Println("Zoom In Button Unpushed")
			res, err := client.Get(conf.URL + "/cgi-bin/ptz.cgi?action=stop&channel=0&code=ZoomTele&arg1=0&arg2=1&arg3=0&arg4=0")
			if err != nil {
				fmt.Printf("Error zooming out: %v\n", err)
				continue
			}
			fmt.Println("Zoom out return: ", res.StatusCode)
			res.Body.Close()
		case e := <-evts[4]:
			x := e.(joysticks.CoordsEvent).X
			y := e.(joysticks.CoordsEvent).Y

			if x > 10000 {

			}

			// Speed is in a range of 1-8
			// http://<ip>/cgi-bin/ptz.cgi?action=[action]&channel=[ch]&code=[code]&arg1=[argstr]& arg2=[argstr]&arg3=[argstr]

			if math.Abs(float64(x)) > deadzone || math.Abs(float64(y)) > deadzone {
				fmt.Printf("X: %v, Y: %v\n", x, y)
			}
		}

	}

}
