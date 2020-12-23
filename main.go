package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/icholy/digest"
	"github.com/splace/joysticks"
	"gopkg.in/yaml.v3"
)

const (
	Up    = "Up"
	Down  = "Down"
	Left  = "Left"
	Right = "Right"
)

type Config struct {
	Cameras []Camera `yaml:"cameras"`
}

type Camera struct {
	User   string       `yaml:"user"`
	Pass   string       `yaml:"pass"`
	URL    string       `yaml:"url"`
	client *http.Client `yaml:"-"`
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

	for i := range conf.Cameras {
		conf.Cameras[i].client = &http.Client{
			Transport: &digest.Transport{
				Username: conf.Cameras[i].User,
				Password: conf.Cameras[i].Pass,
			},
		}
	}

	device := joysticks.Connect(1)

	aPress := device.OnClose(1)
	aUnpress := device.OnOpen(1)
	bPress := device.OnClose(2)
	bUnpress := device.OnOpen(2)
	leftMove := device.OnMove(1)
	dpadMove := device.OnMove(4)

	go device.ParcelOutEvents()

	prevX := 0
	prevY := 0
	activeCamera := conf.Cameras[0]

	for {
		select {
		case <-aPress:
			fmt.Println("Zoom Out Button Pushed")
			res, err := activeCamera.client.Get(activeCamera.URL + "/cgi-bin/ptz.cgi?action=start&channel=0&code=ZoomWide&arg1=0&arg2=1&arg3=0&arg4=0")
			if err != nil {
				fmt.Printf("Error zooming in: %v\n", err)
				continue
			}
			fmt.Println("Zoom in return: ", res.StatusCode)
			res.Body.Close()
		case <-aUnpress:
			fmt.Println("Zoom Out Button Unpushed")
			res, err := activeCamera.client.Get(activeCamera.URL + "/cgi-bin/ptz.cgi?action=stop&channel=0&code=ZoomWide&arg1=0&arg2=1&arg3=0&arg4=0")
			if err != nil {
				fmt.Printf("Error zooming out: %v\n", err)
				continue
			}
			fmt.Println("Zoom out return: ", res.StatusCode)
		case <-bPress:
			fmt.Println("Zoom In Button Pushed")
			res, err := activeCamera.client.Get(activeCamera.URL + "/cgi-bin/ptz.cgi?action=start&channel=0&code=ZoomTele&arg1=0&arg2=1&arg3=0&arg4=0")
			if err != nil {
				fmt.Printf("Error zooming in: %v\n", err)
				continue
			}
			fmt.Println("Zoom in return: ", res.StatusCode)
			res.Body.Close()
		case <-bUnpress:
			fmt.Println("Zoom In Button Unpushed")
			res, err := activeCamera.client.Get(activeCamera.URL + "/cgi-bin/ptz.cgi?action=stop&channel=0&code=ZoomTele&arg1=0&arg2=1&arg3=0&arg4=0")
			if err != nil {
				fmt.Printf("Error zooming out: %v\n", err)
				continue
			}
			fmt.Println("Zoom out return: ", res.StatusCode)
			res.Body.Close()
		case e := <-leftMove:
			// Joystick range is -1 to +1, multiply by 10 to make maths a little easier for my brain
			// round to integer for more coarse stepping
			x := int(math.Round(float64(e.(joysticks.CoordsEvent).X * 10)))
			y := int(math.Round(float64(e.(joysticks.CoordsEvent).Y * 10)))

			if x != prevX {
				prevX = x
				if math.Abs(float64(x)) < 2 {
					ptzStop(conf, &activeCamera)
					fmt.Println("Pan Stopped")
				}
				if x > 2 {
					spd := strconv.Itoa(x - 2)
					ptzMove(conf, &activeCamera, Right, spd)
					fmt.Println("Panning right", spd)
				}
				if x < -2 {
					spd := strconv.Itoa(int(math.Abs(float64(x + 2))))
					ptzMove(conf, &activeCamera, Left, spd)
					fmt.Println("Panning left", spd)
				}
			}

			if y != prevY {
				prevY = y
				if math.Abs(float64(y)) < 2 {
					ptzStop(conf, &activeCamera)
					fmt.Println("Tilt Stopped")
				}
				if y > 2 {
					spd := strconv.Itoa(y - 2)
					ptzMove(conf, &activeCamera, Down, spd)
					fmt.Println("Tilt down", spd)
				}
				if y < -2 {
					spd := strconv.Itoa(int(math.Abs(float64(y + 2))))
					ptzMove(conf, &activeCamera, Up, spd)
					fmt.Println("Tilt up", spd)
				}
			}
		case e := <-dpadMove:
			// Joystick range is -1 to +1, multiply by 10 to make maths a little easier for my brain
			// round to integer for more coarse stepping
			x := int(math.Round(float64(e.(joysticks.CoordsEvent).X * 10)))
			y := int(math.Round(float64(e.(joysticks.CoordsEvent).Y * 10)))

			if math.Abs(float64(x)) < 2 && math.Abs(float64(y)) < 2 {
				ptzStop(conf, &activeCamera)
				fmt.Println("Pan/Tilt Stopped")
			}
			if x > 2 {
				ptzMove(conf, &activeCamera, Right, "1")
				fmt.Println("Panning right", "1")
			}
			if x < -2 {
				ptzMove(conf, &activeCamera, Left, "1")
				fmt.Println("Panning left", "1")
			}

			if y > 2 {
				ptzMove(conf, &activeCamera, Down, "1")
				fmt.Println("Tilt down", "1")
			}
			if y < -2 {
				ptzMove(conf, &activeCamera, Up, "1")
				fmt.Println("Tilt up", "1")
			}
		}
	}

}

func ptzStop(conf *Config, activeCamera *Camera) {
	res, err := activeCamera.client.Get(activeCamera.URL + "/cgi-bin/ptz.cgi?action=stop&channel=0&code=Down&arg1=0&arg2=1&arg3=0&arg4=0")
	if err != nil {
		fmt.Printf("Error moving camera: %v\n", err)
		return
	}
	defer res.Body.Close()
}

func ptzMove(conf *Config, activeCamera *Camera, dir, spd string) {
	res, err := activeCamera.client.Get(activeCamera.URL + "/cgi-bin/ptz.cgi?action=start&channel=0&code=" + dir + "&arg1=0&arg2=" + spd + "&arg3=0&arg4=0")
	if err != nil {
		fmt.Printf("Error moving camera: %v\n", err)
		return
	}
	defer res.Body.Close()
}
