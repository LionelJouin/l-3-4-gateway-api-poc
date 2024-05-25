/*
Copyright (c) 2024 OpenInfra Foundation Europe

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cni

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"golang.org/x/sys/unix"
)

const (
	defaultDataDir     = "/var/lib/cni/store"
	dataDirPermission  = 0o755
	dataFilePermission = 0o644
)

// Store saves the cni stdin data to a file a CNI request (unique tuple composed
// of the network namespace, network name and interface name).
func Store(args *skel.CmdArgs) error {
	filename, err := getFilename(args)
	if err != nil {
		return err
	}

	filePath := filepath.Join(defaultDataDir, filename)

	err = os.MkdirAll(defaultDataDir, dataDirPermission)
	if err != nil {
		return fmt.Errorf("failed to create the directory %s: %w", defaultDataDir, err)
	}

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_EXCL|os.O_CREATE, dataFilePermission)
	if err != nil {
		return fmt.Errorf("failed to create the file %s: %w", filePath, err)
	}

	_, err = file.Write(args.StdinData)
	if err != nil {
		return fmt.Errorf("failed to write in the file %s: %w", filePath, err)
	}

	err = file.Close()
	if err != nil {
		return fmt.Errorf("failed to close the file %s: %w", filePath, err)
	}

	return nil
}

// Get retrieves the cni stdin data for a CNI request from a previously saved file
// (unique tuple composed of the network namespace, network name and interface name).
func Get(args *skel.CmdArgs) ([]byte, error) {
	filename, err := getFilename(args)
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(defaultDataDir, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read the file %s: %w", filePath, err)
	}

	return data, nil
}

// Delete removes the file with the cni stdin data for a CNI request (unique tuple
// composed of the network namespace, network name and interface name).
func Delete(args *skel.CmdArgs) error {
	filename, err := getFilename(args)
	if err != nil {
		return err
	}

	filePath := filepath.Join(defaultDataDir, filename)

	err = os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("failed to remove the file %s: %w", filePath, err)
	}

	return nil
}

func getFilename(args *skel.CmdArgs) (string, error) {
	conf := &types.NetConf{}

	err := json.Unmarshal(args.StdinData, conf)
	if err != nil {
		return "", fmt.Errorf("failed to load netconf: %w", err)
	}

	netnsID, err := getNetworkNamespaceID(args.Netns)
	if err != nil {
		return "", fmt.Errorf("failed to get netns ID %q: %w", args.Netns, err)
	}

	return fmt.Sprintf("%s-%s-%s.json", netnsID, conf.Name, args.IfName), nil
}

func getNetworkNamespaceID(netns string) (string, error) {
	fd, err := unix.Open(netns, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open netns %q: %w", netns, err)
	}

	defer func() {
		_ = unix.Close(fd)
	}()

	var fileInformation unix.Stat_t

	err = unix.Fstat(fd, &fileInformation)
	if err != nil {
		return "", fmt.Errorf("failed to stats the netns fd %q: %w", netns, err)
	}

	return fmt.Sprintf("%d-%d", fileInformation.Dev, fileInformation.Ino), nil
}
