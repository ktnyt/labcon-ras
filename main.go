package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ktnyt/labcon"
	"github.com/ktnyt/labcon/driver"
)

type Station struct {
	Driver labcon.Driver
	Spots  []bool
}

func NewStation(client *labcon.Client, i, n int) Station {
	spots := make([]bool, n)
	driver, err := labcon.NewDriver(client, fmt.Sprintf("station%d", i), spots)
	if err != nil {
		log.Fatal(err)
	}
	return Station{
		Driver: driver,
		Spots:  spots,
	}
}

type ArmArg struct {
	Station int `json:"station"`
	Spot    int `json:"spot"`
}

func Convert(dst interface{}, src interface{}) error {
	var network bytes.Buffer
	enc := json.NewEncoder(&network)
	dec := json.NewDecoder(&network)
	if err := enc.Encode(src); err != nil {
		return err
	}
	return dec.Decode(dst)
}

func main() {
	host := os.Getenv("HOST")
	port := os.Getenv("PORT")
	if host == "" {
		host = "http://localhost"
	}
	if port == "" {
		port = "5000"
	}
	addr := fmt.Sprintf("%s:%s", host, port)

	client := labcon.NewClient(addr)
	arm, err := labcon.NewDriver(client, "arm", false)
	if err != nil {
		log.Fatal(err)
	}

	config := []int{2, 1, 1}
	stations := make([]Station, len(config))
	for i, n := range config {
		stations[i] = NewStation(client, i, n)
	}

	stations[0].Spots[0] = true

	ticker := time.NewTicker(time.Second)
	done := make(chan error)

	go func() {
		for {
			select {
			case <-done:
				return

			case <-ticker.C:
				op, err := arm.Operation()
				if err != nil {
					done <- err
					return
				}

				if op != nil {

					switch op.Name {
					case "take", "put":
						var arg ArmArg
						if err := Convert(&arg, op.Arg); err != nil {
							status := driver.Status(fmt.Sprintf("bad argument for operation %q: %v", op.Name, err))
							if err := arm.SetStatus(status); err != nil {
								done <- err
								return
							}
						}

						log.Printf("%s: station %d, spot %d", op.Name, arg.Station, arg.Spot)

						if stations[arg.Station].Spots[arg.Spot] == (op.Name == "take") {
							status := driver.Status(fmt.Sprintf("no sample to take at station %d, spot %d", arg.Station, arg.Spot))
							if err := arm.SetStatus(status); err != nil {
								done <- err
								return
							}
						}

						time.Sleep(2 * time.Second)

						stations[arg.Station].Spots[arg.Spot] = op.Name == "put"
						if err := stations[arg.Station].Driver.SetState(stations[arg.Station].Spots); err != nil {
							done <- err
							return
						}

						if err := arm.SetState(op.Name == "take"); err != nil {
							done <- err
							return
						}

						if err := arm.SetStatus(driver.Idle); err != nil {
							done <- err
							return
						}

					case "reboot":
						log.Println("reboot")
						if err := arm.SetStatus(driver.Idle); err != nil {
							done <- err
							return
						}

					default:
						status := driver.Status(fmt.Sprintf("error: unknown operation %q", op.Name))
						if err := arm.SetStatus(status); err != nil {
							done <- err
							return
						}
					}
				}
			}
		}
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(
		sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	for {
		select {
		case <-sigc:
			done <- nil
			return

		case err := <-done:
			if err != nil {
				log.Fatal(err)
			}
			ticker.Stop()
			return
		}
	}
}
