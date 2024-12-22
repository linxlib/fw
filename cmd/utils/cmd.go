package utils

import (
	"bufio"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"os/exec"
)

func RunCmd(cmd string, args ...string) error {
	command := exec.Command(cmd, args...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	go func() {
		decoder := transform.NewReader(stdout, simplifiedchinese.GBK.NewDecoder())
		scanner := bufio.NewScanner(decoder)
		for scanner.Scan() {
			fmt.Printf("%s\n", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading from StdoutPipe: %v\n", err)
		}
	}()
	go func() {
		decoder := transform.NewReader(stderr, simplifiedchinese.GBK.NewDecoder())
		scanner := bufio.NewScanner(decoder)
		for scanner.Scan() {
			fmt.Printf("%s\n", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("Error reading from StderrPipe: %v\n", err)
		}
	}()
	if err := command.Wait(); err != nil {
		return err
	}
	return nil
}
