package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/icholy/digest"
	"github.com/splace/joysticks"
	"github.com/use-go/onvif"
	"github.com/use-go/onvif/ptz"
	onvif2 "github.com/use-go/onvif/xsd/onvif"
	"gopkg.in/yaml.v3"
)

const (
	ptzSpeedScale = 0.25
)

type Config struct {
	Cameras []Camera `yaml:"cameras"`
}

type Camera struct {
	User       string        `yaml:"user"`
	Pass       string        `yaml:"pass"`
	URL        string        `yaml:"url"`
	ONVIFToken string        `yaml:"onvif_profile_token"`
	client     *http.Client  `yaml:"-"`
	dev        *onvif.Device `yaml:"-"`
}

func main() {
	showJoystick := flag.Bool("show-joystick", false, "print the buttons and axis from a joystick")
	flag.Parse()

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
		dev, err := onvif.NewDevice(conf.Cameras[i].URL)
		if err != nil {
			fmt.Println("Error opening onvif device:", err)
			continue
		}
		dev.Authenticate(conf.Cameras[i].User, conf.Cameras[i].Pass)
		//resp, err := dev.CallMethod(media.GetProfiles{})
		//if err != nil {
		//	log.Println(err)
		//} else {
		//	fmt.Println(readResponse(resp))
		//}
		conf.Cameras[i].dev = dev
		conf.Cameras[i].client = &http.Client{
			Transport: &digest.Transport{
				Username: conf.Cameras[i].User,
				Password: conf.Cameras[i].Pass,
			},
		}
	}

	joystick := joysticks.Connect(1)
	go joystick.ParcelOutEvents()

	if *showJoystick {
		t := time.NewTicker(2 * time.Second)
		for {
			select {
			case <-t.C:
				fmt.Printf("Buttons: %v\n", joystick.Buttons)
				fmt.Printf("Axis: %v\n", joystick.HatAxes)
				//case e := <-device.OSEvents:
				//	fmt.Printf("OS Event: %v\n", e)
			}
		}
	}

	aPress := joystick.OnClose(1)
	aUnpress := joystick.OnOpen(1)
	bPress := joystick.OnClose(2)
	bUnpress := joystick.OnOpen(2)
	xPress := joystick.OnClose(3)
	leftMove := joystick.OnMove(1)
	dpadMove := joystick.OnMove(4)

	prevX := 0.0
	prevY := 0.0
	activeCameraIdx := 0
	activeCamera := conf.Cameras[0]

	fmt.Println("Running...")
	for {
		select {
		case <-aPress:
			fmt.Println("Zoom Out Button Pushed")
			zi := ptz.ContinuousMove{
				ProfileToken: onvif2.ReferenceToken(activeCamera.ONVIFToken),
				Velocity: onvif2.PTZSpeed{
					Zoom: onvif2.Vector1D{
						X: -0.8,
					},
				},
				Timeout: "PT10S",
			}
			resp, err := activeCamera.dev.CallMethod(zi)
			if err != nil {
				log.Println(err)
			} else {
				fmt.Println(readResponse(resp))
			}
		case <-aUnpress:
			fmt.Println("Zoom Out Button Unpushed")
			zs := ptz.Stop{
				ProfileToken: onvif2.ReferenceToken(activeCamera.ONVIFToken),
				PanTilt:      true,
				Zoom:         true,
			}
			resp, err := activeCamera.dev.CallMethod(zs)
			if err != nil {
				log.Println(err)
			} else {
				fmt.Println(readResponse(resp))
			}
		case <-bPress:
			fmt.Println("Zoom In Button Pushed")
			zi := ptz.ContinuousMove{
				ProfileToken: onvif2.ReferenceToken(activeCamera.ONVIFToken),
				Velocity: onvif2.PTZSpeed{
					Zoom: onvif2.Vector1D{
						X: 0.8,
					},
				},
				Timeout: "PT10S",
			}
			resp, err := activeCamera.dev.CallMethod(zi)
			if err != nil {
				log.Println(err)
			} else {
				fmt.Println(readResponse(resp))
			}
		case <-bUnpress:
			fmt.Println("Zoom In Button Unpushed")
			zs := ptz.Stop{
				ProfileToken: onvif2.ReferenceToken(activeCamera.ONVIFToken),
				PanTilt:      true,
				Zoom:         true,
			}
			resp, err := activeCamera.dev.CallMethod(zs)
			if err != nil {
				log.Println(err)
			} else {
				fmt.Println(readResponse(resp))
			}
		case <-xPress:
			fmt.Println("Changing camera")
			activeCameraIdx++
			if activeCameraIdx >= len(conf.Cameras) {
				activeCameraIdx = 0
			}
			activeCamera = conf.Cameras[activeCameraIdx]
			fmt.Println("New camera idx:", activeCameraIdx)
		case e := <-leftMove:
			// Joystick range is -1 to +1, multiply by 10 to make maths a little easier for my brain
			// round to integer for more coarse stepping
			x := math.Round(float64(e.(joysticks.CoordsEvent).X * 10))
			y := math.Round(float64(e.(joysticks.CoordsEvent).Y * 10))

			if x != prevX {
				prevX = x
				if math.Abs(float64(x)) < 2 {
					ptzStop(&activeCamera)
					fmt.Println("Pan Stopped")
				}
				if x > 2 {
					ptMove(&activeCamera, x/10*ptzSpeedScale, -y/10*ptzSpeedScale)
					fmt.Println("Panning right")
				}
				if x < -2 {
					ptMove(&activeCamera, x/10*ptzSpeedScale, -y/10*ptzSpeedScale)
					fmt.Println("Panning left")
				}
			}

			if y != prevY {
				prevY = y
				if math.Abs(float64(y)) < 2 {
					ptzStop(&activeCamera)
					fmt.Println("Tilt Stopped")
				}
				if y > 2 {
					ptMove(&activeCamera, x/10*ptzSpeedScale, -y/10*ptzSpeedScale)
					fmt.Println("Tilt down")
				}
				if y < -2 {
					ptMove(&activeCamera, x/10*ptzSpeedScale, -y/10*ptzSpeedScale)
					fmt.Println("Tilt up")
				}
			}
		case e := <-dpadMove:
			// Joystick range is -1 to +1, multiply by 10 to make maths a little easier for my brain
			// round to integer for more coarse stepping
			x := int(math.Round(float64(e.(joysticks.CoordsEvent).X * 10)))
			y := int(math.Round(float64(e.(joysticks.CoordsEvent).Y * 10)))

			if math.Abs(float64(x)) < 2 && math.Abs(float64(y)) < 2 {
				ptzStop(&activeCamera)
				fmt.Println("Pan/Tilt Stopped")
			}
			if x > 2 {
				ptMove(&activeCamera, 0.1, 0)
				fmt.Println("Panning right", "1")
			}
			if x < -2 {
				ptMove(&activeCamera, -0.1, 0)
				fmt.Println("Panning left", "1")
			}

			if y > 2 {
				ptMove(&activeCamera, 0, -0.1)
				fmt.Println("Tilt down", "1")
			}
			if y < -2 {
				ptMove(&activeCamera, 0, 0.1)
				fmt.Println("Tilt up", "1")
			}
		}
	}

}

func readResponse(resp *http.Response) string {
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func ptzStop(activeCamera *Camera) {
	zs := ptz.Stop{
		ProfileToken: onvif2.ReferenceToken(activeCamera.ONVIFToken),
		PanTilt:      true,
		Zoom:         true,
	}
	resp, err := activeCamera.dev.CallMethod(zs)
	if err != nil {
		log.Println(err)
	} else {
		fmt.Println(readResponse(resp))
	}
}

func ptMove(activeCamera *Camera, x, y float64) {
	mv := ptz.ContinuousMove{
		ProfileToken: onvif2.ReferenceToken(activeCamera.ONVIFToken),
		Velocity: onvif2.PTZSpeed{
			PanTilt: onvif2.Vector2D{
				X: x,
				Y: y,
			},
		},
		Timeout: "PT10S",
	}
	resp, err := activeCamera.dev.CallMethod(mv)
	if err != nil {
		log.Println(err)
	} else {
		fmt.Println(readResponse(resp))
	}
}
