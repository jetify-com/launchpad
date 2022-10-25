package main

import (
	"context"
	"fmt"

	"go.jetpack.io/launchpad/pkg/reaktor"
	"go.jetpack.io/launchpad/pkg/reaktor/komponents"
)

// Example using reaktor
func main() {
	// 1. Initialize a new klient
	klient, err := reaktor.New()
	if err != nil {
		panic(err)
	}

	// 2. Define a simple job
	job := &komponents.Job{
		Name:           "test-job",
		Namespace:      "test-namespace",
		ContainerImage: "alpine",
		Command:        []string{"echo", "This is a job!"},
	}

	// 3. Deploy to the k8s cluster
	fmt.Println("Creating job...")
	result, err := klient.Apply(context.Background(), job)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created job %q.\n", result.GetName())
}
