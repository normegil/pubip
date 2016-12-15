// Package ipify provides a single function for retrieving your computer's
// public IP address from the ipify service: http://www.ipify.org
package pubip

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jpillora/backoff"
)

// GetIp queries the ipify service (http://www.ipify.org) to retrieve this
// machine's public IP address.  Returns your public IP address as a string, and
// any error encountered.  By default, this function will run using exponential
// backoff -- if this function fails for any reason, the request will be retried
// up to 3 times.
//
// Usage:
//
//		package main
//
//		import (
//			"fmt"
//			"github.com/rdegges/go-ipify"
//		)
//
//		func main() {
//			ip, err := ipify.GetIp()
//			if err != nil {
//				fmt.Println("Couldn't get my IP address:", err)
//			} else {
//				fmt.Println("My IP address is:", ip)
//			}
//		}
func GetIpBy(dest string) (string, error) {
	b := &backoff.Backoff{
		Jitter: true,
	}
	client := &http.Client{}

	req, err := http.NewRequest("GET", dest, nil)
	if err != nil {
		return "", err
	}

	for tries := 0; tries < MaxTries; tries++ {
		resp, err := client.Do(req)
		if err != nil {
			d := b.Duration()
			time.Sleep(d)
			continue
		}

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}

		if resp.StatusCode != 200 {
			return "", errors.New(dest + " status code " + strconv.Itoa(resp.StatusCode) + ", body: " + string(body))
		}

		tb := strings.TrimSpace(string(body))
		ip := net.ParseIP(tb)
		if ip == nil {
			return "", errors.New("IP address not valid: " + tb)
		}
		return ip.String(), nil
	}

	return "", errors.New("Failed to reach " + dest)
}

func detailErr(err error, errs []error) error {
	errStrs := []string{err.Error()}
	for _, e := range errs {
		errStrs = append(errStrs, e.Error())
	}
	j := strings.Join(errStrs, "\n")
	return errors.New(j)
}

func validate(rs []string) (string, error) {
	if len(rs) < 3 {
		return "", fmt.Errorf("Less than %d results from %d APIs", 3, len(APIURIs))
	}
	first := rs[0]
	for i := 1; i < len(rs); i++ {
		if first != rs[i] {
			return "", fmt.Errorf("Results are not identical: %s", rs)
		}
	}
	return first, nil
}

func worker(d string, r chan<- string, e chan<- error) {
	ip, err := GetIpBy(d)
	if err != nil {
		e <- err
		return
	}
	r <- ip
}

func Get() (string, error) {
	var results []string
	resultCh := make(chan string, len(APIURIs))
	var errs []error
	errCh := make(chan error, len(APIURIs))

	for _, d := range APIURIs {
		go worker(d, resultCh, errCh)
	}
	for {
		select {
		case err := <-errCh:
			errs = append(errs, err)
		case r := <-resultCh:
			results = append(results, r)
		case <-time.After(Timeout):
			r, err := validate(results)
			if err != nil {
				return "", detailErr(err, errs)
			}
			return r, nil
		}
	}
}
