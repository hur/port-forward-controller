/*
Copyright 2025.

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

package forwarding

import (
	"context"
	"reflect"
	"strings"
)

type PortForward struct {
	Name    string
	Address string
	Port    int32
}

type Client interface {
	CreatePortForwards(ctx context.Context, forwards []PortForward) error
	ListPortForwards(ctx context.Context) ([]PortForward, error)
	DeletePortForwards(ctx context.Context, forwards []PortForward) error
}

type ForwardingReconciler struct {
	Client     Client
	RulePrefix string
}

func (fr *ForwardingReconciler) EnsureAddresses(ctx context.Context, addresses []PortForward) error {
	existingAddresses, err := fr.Client.ListPortForwards(ctx)
	if err != nil {
		return err
	}

	staleAddresses := fr.staleAddresses(addresses, existingAddresses)
	err = fr.Client.DeletePortForwards(ctx, staleAddresses)
	if err != nil {
		return err
	}

	missingAddresses := fr.missingAddresses(addresses, existingAddresses)
	return fr.Client.CreatePortForwards(ctx, missingAddresses)
}

func (fr *ForwardingReconciler) DeleteAddresses(ctx context.Context, addresses []PortForward) error {
	existingAddresses, err := fr.Client.ListPortForwards(ctx)
	if err != nil {
		return err
	}

	addressesToDelete := []PortForward{}
	for _, address := range addresses {
		for _, existingAddress := range existingAddresses {
			if reflect.DeepEqual(address, existingAddress) {
				addressesToDelete = append(addressesToDelete, existingAddress)
				break
			}
		}
	}

	return fr.Client.DeletePortForwards(ctx, addressesToDelete)
}

func (fr *ForwardingReconciler) missingAddresses(desiredAddresses []PortForward, existingAddresses []PortForward) []PortForward {
	missingAddresses := []PortForward{}

	for _, desiredAddress := range desiredAddresses {
		missing := true
		for _, address := range existingAddresses {
			if reflect.DeepEqual(address, desiredAddress) {
				missing = false
				break
			}
		}

		if missing {
			missingAddresses = append(missingAddresses, desiredAddress)
		}
	}

	return missingAddresses
}

func (fr *ForwardingReconciler) staleAddresses(desiredAddresses []PortForward, existingAddresses []PortForward) []PortForward {
	staleAddresses := []PortForward{}

	for _, address := range existingAddresses {
		match := false
		for _, desiredAddress := range desiredAddresses {
			if strings.HasPrefix(address.Name, desiredAddress.Name) {
				if reflect.DeepEqual(address, desiredAddress) {
					match = true
					break
				}
			}
		}

		// A currently existing address that is no longer desired is considered "stale"
		if !match {
			staleAddresses = append(staleAddresses, address)
		}
	}
	return staleAddresses
}
