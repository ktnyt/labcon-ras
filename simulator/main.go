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
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
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
	w := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}
	logger := zlog.Output(w).Level(zerolog.TraceLevel)

	host := os.Getenv("HOST")
	port := os.Getenv("PORT")
	if host == "" {
		host = "http://localhost"
	}
	if port == "" {
		port = "5000"
	}
	addr := fmt.Sprintf("%s:%s", host, port)

	logger.Info().Msg("Setup Arm Driver")
	client := labcon.NewClient(addr)
	arm, err := labcon.NewDriver(client, "arm", false)
	if err != nil {
		log.Fatal(err)
	}
	spot := false

	defer func() {
		logger.Info().Msg("Disconnect Arm")
		arm.Disconnect()
	}()

	logger.Info().Msg("Setup Station Drivers")
	config := []int{2, 1, 1}
	stations := make([]Station, len(config))
	for i, n := range config {
		stations[i] = NewStation(client, i, n)
	}

	defer func() {
		for i, station := range stations {
			logger.Info().Msgf("Disconnect Station %d", i)
			station.Driver.Disconnect()
		}
	}()

	stations[0].Spots[0] = true
	stations[0].Driver.SetState(stations[0].Spots)

	logger.Info().Msg("Listen for dispatch")

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
					logger.Info().Str("op", op.Name).Msg("Received Operation")

					switch op.Name {
					case "take":
						if spot {
							status := driver.Status("arm already has a sample")
							if err := arm.SetStatus(status); err != nil {
								done <- err
								return
							}
						}

						var arg ArmArg
						if err := Convert(&arg, op.Arg); err != nil {
							status := driver.Status(fmt.Sprintf("bad argument for operation %q: %v", op.Name, err))
							if err := arm.SetStatus(status); err != nil {
								done <- err
								return
							}
						}

						logger.Info().Msgf("take: station %d, spot %d", arg.Station, arg.Spot)

						time.Sleep(5 * time.Second)

						if !stations[arg.Station].Spots[arg.Spot] {
							status := driver.Status(fmt.Sprintf("no sample to take at station %d, spot %d", arg.Station, arg.Spot))
							if err := arm.SetStatus(status); err != nil {
								done <- err
								return
							}
						}

						stations[arg.Station].Spots[arg.Spot] = false
						if err := stations[arg.Station].Driver.SetState(stations[arg.Station].Spots); err != nil {
							done <- err
							return
						}

						spot = true
						if err := arm.SetState(spot); err != nil {
							done <- err
							return
						}

						if err := arm.SetStatus(driver.Idle); err != nil {
							done <- err
							return
						}

					case "put":
						if !spot {
							status := driver.Status("arm odes not have a sample")
							if err := arm.SetStatus(status); err != nil {
								done <- err
								return
							}
						}

						var arg ArmArg
						if err := Convert(&arg, op.Arg); err != nil {
							status := driver.Status(fmt.Sprintf("bad argument for operation %q: %v", op.Name, err))
							if err := arm.SetStatus(status); err != nil {
								done <- err
								return
							}
						}

						logger.Info().Msgf("put: station %d, spot %d", arg.Station, arg.Spot)

						time.Sleep(5 * time.Second)

						if !stations[arg.Station].Spots[arg.Spot] {
							status := driver.Status(fmt.Sprintf("sample is present at station %d, spot %d", arg.Station, arg.Spot))
							if err := arm.SetStatus(status); err != nil {
								done <- err
								return
							}
						}

						stations[arg.Station].Spots[arg.Spot] = true
						if err := stations[arg.Station].Driver.SetState(stations[arg.Station].Spots); err != nil {
							done <- err
							return
						}

						spot = false
						if err := arm.SetState(spot); err != nil {
							done <- err
							return
						}

						if err := arm.SetStatus(driver.Idle); err != nil {
							done <- err
							return
						}

					case "reboot":
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
