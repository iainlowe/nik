package main

/*

#include <unistd.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
#include <string.h>
#include <fcntl.h>

// Writes the supplied hostfile buffer to /etc/hosts
// in the mnt namespace of the supplied nspid
void sethostfile(int nspid, char* hostfile) {
	int pid = fork();

	if (pid == 0) { // in the child; change ns and write file
		char* _nspath = malloc(1024);
		sprintf(_nspath, "/proc/%d/ns/mnt", nspid);

		char* nspath = (char*) malloc(strlen(_nspath) + 1);
		strcpy(nspath, _nspath);

		free(_nspath);
		int fd = open(nspath, O_RDONLY, 0644);
		free(nspath);

		int setns_r = setns(fd, 0);
		close(fd);

		if (setns_r == -1) {
			fprintf(stderr, "setns failed: %s\n", strerror(errno));
			exit(2);
		}

		int hfd = open("/etc/hosts", O_RDWR, 0644);

		if (hfd == -1) {
			if (errno == 30) {
				fprintf(stderr, "failed to open /etc/hosts in write mode; are you running Docker 1.2+?\nCheck with 'docker version' and try again.\n");
				exit(99);
			} else {
				fprintf(stderr, "failed to open /etc/hosts: %s\n", strerror(errno));
			}
		}

		if (ftruncate(hfd, 0) != 0) {
			fprintf(stderr, "failed to truncate hostfile: %s\n", strerror(errno));
		}

		int bytes = write(hfd, hostfile, strlen(hostfile) + 1);
		close(hfd);

		if (bytes != strlen(hostfile) + 1) {
			fprintf(stderr, "warning: written byte count did not match (wrote %d but file was %d)", bytes, (int) strlen(hostfile) + 1);
			exit(3);
		}

		exit(0);
	} else { // in the parent, wait for the child to return
		int status;
		waitpid(pid, &status, 0);
		errno = status;

		if (errno != 0) exit(errno);
	}

	return;
}


*/
import "C"

import (
	"bytes"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"encoding/hex"
	"encoding/json"
	"path/filepath"
)

type CConfig struct {
	Hostname string
}

type CNetworkSettings struct {
	IPAddress string
}

type CState struct {
	Running bool
	Pid     int
}

type Container struct {
	Id string

	Config          CConfig
	NetworkSettings CNetworkSettings
	State           CState
}

func dockerbytes(url string) ([]byte, error) {
	conn, err := net.Dial("unix", "/var/run/docker.sock")
	defer conn.Close()

	if err != nil {
		return nil, err
	}

	b := make([]byte, 8000)

	conn.Write([]byte(strings.Join([]string{"GET", url, "HTTP/1.0\n\n"}, " ")))
	n, _ := conn.Read(b)

	b = b[:n]

	idx := strings.Index(string(b), "\r\n\r\n") + 4

	if n == 8000 {
		panic("READ BUFFER FULL")
	}

	return b[idx:], nil
}

func getContainers() ([]Container, error) {
	var containers []Container
	b, err := dockerbytes("/containers/json")
	err = json.Unmarshal(b, &containers)

	if err != nil {
		return nil, err
	}

	r := []Container{}

	for i := len(containers) - 1; i > -1; i-- {
		c := containers[i]

		b, err := dockerbytes(filepath.Join("/containers", c.Id, "json"))

		if err != nil {
			return r, err
		}

		var cc Container
		err = json.Unmarshal(b, &cc)

		if err != nil {
			return r, err
		}

		r = append(r, cc)
	}

	return r, nil
}

func updateContainers(hash string) string {
	cs, err := getContainers()

	if err != nil {
		log.Fatalln(err)
	}

	var buf bytes.Buffer

	buf.WriteString("##\n# /etc/hosts file AUTO-GENERATED by nik\n#\n\n127.0.0.1 localhost\n\n")

	for i := range cs {
		c := cs[i]

		buf.WriteString(strings.Join([]string{c.NetworkSettings.IPAddress, c.Config.Hostname}, " "))
		buf.WriteString("\n")
	}

	s := buf.String()

	h := fnv.New64()
	h.Write(buf.Bytes())
	hstr := hex.EncodeToString(h.Sum(nil))

	if hstr == hash {
		return hstr
	}

	ss := C.CString(s)

	for i := range cs {
		c := cs[i]

		C.sethostfile(C.int(c.State.Pid), ss)
	}

	return hstr
}

var (
	Debug, Info, Warn, Error *log.Logger
)

func initLogging(debug, info, warn, err io.Writer) {
	Debug = log.New(debug, "DEBUG: ", log.Ldate|log.Ltime)
	Info = log.New(info, "INFO: ", log.Ldate|log.Ltime)
	Warn = log.New(warn, "WARN: ", log.Ldate|log.Ltime)
	Error = log.New(err, "ERROR: ", log.Ldate|log.Ltime)
}

func main() {
	var interval int
	var verbose bool
	var quiet bool

	flag.IntVar(&interval, "i", 5, "polling interval")
	flag.BoolVar(&quiet, "q", false, "be quiet")
	flag.BoolVar(&verbose, "v", false, "be verbose")

	flag.Parse()

	if verbose {
		initLogging(os.Stdout, os.Stdout, os.Stdout, os.Stdout)
	} else if quiet {
		initLogging(ioutil.Discard, ioutil.Discard, ioutil.Discard, ioutil.Discard)
	} else {
		initLogging(ioutil.Discard, os.Stdout, os.Stderr, os.Stderr)
	}

	if os.Geteuid() != 0 {
		fmt.Println("abort: need to be root")
		os.Exit(1)
	}

	var hash string

	for {
		hash = updateContainers(hash)
		Info.Println("processed update with hash:", hash)
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
