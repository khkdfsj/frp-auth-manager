//go:build windows

package main

import (
	"context"
	"log"
	"time"

	"golang.org/x/sys/windows/svc"
)

type windowsService struct {
	configPath string
}

func maybeRunWindowsService(configPath string) (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false, err
	}
	if !isService {
		return false, nil
	}
	return true, svc.Run(serviceName, &windowsService{configPath: configPath})
}

func (s *windowsService) Execute(args []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, s.configPath)
	}()

	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				changes <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				select {
				case err := <-errCh:
					if err != nil {
						log.Printf("service stopped with error: %v", err)
					}
				case <-time.After(8 * time.Second):
				}
				return false, 0
			default:
				continue
			}
		case err := <-errCh:
			if err != nil {
				log.Printf("agent exited: %v", err)
				return true, 1
			}
			return false, 0
		}
	}
}
