//go:build mage

// This magefile contains targets for starting and stopping
// the Sakila test containers locally.

package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/magefile/mage/mg"
	"golang.org/x/sync/errgroup"

	"github.com/testcontainers/testcontainers-go"
)

// Sakila is the mage namespace for sakila targets.
type Sakila mg.Namespace

const startupTimeout = time.Minute * 5

// requests is the set of database server containers required for
// integration testing.
//
// See: https://golang.testcontainers.org/features/creating_container/
//
// FIXME: huzzah
// TODO: The wait.ForLog mechanism is probably not fully correct for any of the
// containers (e.g. the string may appear multiple times in the log output),
// thus the containers may not have started fully when WaitingFor completes.
var requests = []testcontainers.ContainerRequest{
	//{
	//	Image:        "sakiladb/sqlserver:2017",
	//	Name:         "sakiladb-sqlserver-2017",
	//	ExposedPorts: []string{"14337:1433"},
	//	WaitingFor:   wait.ForLog("Changed database context to 'sakila'.").WithStartupTimeout(startupTimeout),
	//},
	// {
	// 	Image:        "sakiladb/postgres:9",
	// 	Name:         "sakiladb-postgres-9",
	// 	ExposedPorts: []string{"54329:5432"},
	// 	WaitingFor:   wait.ForLog("PostgreSQL init process complete; ready for start up.").WithStartupTimeout(startupTimeout),
	// },
	// {
	// 	Image:        "sakiladb/postgres:10",
	// 	Name:         "sakiladb-postgres-10",
	// 	ExposedPorts: []string{"54330:5432"},
	// 	WaitingFor:   wait.ForLog("PostgreSQL init process complete; ready for start up.").WithStartupTimeout(startupTimeout),
	// },
	// {
	// 	Image:        "sakiladb/postgres:11",
	// 	Name:         "sakiladb-postgres-11",
	// 	ExposedPorts: []string{"54331:5432"},
	// 	WaitingFor:   wait.ForLog("PostgreSQL init process complete; ready for start up.").WithStartupTimeout(startupTimeout),
	// },
	{
		Image:        "sakiladb/postgres:12",
		Name:         "sakiladb-postgres-12",
		ExposedPorts: []string{"54332:5432"},
		WaitingFor:   wait.ForLog("PostgreSQL init process complete; ready for start up.").WithStartupTimeout(startupTimeout),
	},
	// {
	// 	Image:        "sakiladb/mysql:5.6",
	// 	Name:         "sakiladb-mysql-5.6",
	// 	ExposedPorts: []string{"33066:3306"},
	// 	WaitingFor:   wait.ForLog("[Note] mysqld: ready for connections.").WithStartupTimeout(startupTimeout),
	// },
	// {
	// 	Image:        "sakiladb/mysql:5.7",
	// 	Name:         "sakiladb-mysql-5.7",
	// 	ExposedPorts: []string{"33067:3306"},
	// 	WaitingFor:   wait.ForLog("[Note] mysqld: ready for connections.").WithStartupTimeout(startupTimeout),
	// },
	// {
	// 	Image:        "sakiladb/mysql:8",
	// 	Name:         "sakiladb-mysql-8",
	// 	ExposedPorts: []string{"33068:3306"},
	// 	WaitingFor:   wait.ForLog("[Server] /usr/sbin/mysqld: ready for connections.").WithStartupTimeout(startupTimeout),
	// },
}

var startedContainers []testcontainers.Container
var startedContainersMu sync.Mutex

func containerStarted(cont testcontainers.Container) {
	startedContainersMu.Lock()
	defer startedContainersMu.Unlock()

	startedContainers = append(startedContainers, cont)
}

// StartAll starts all the sakila database server containers locally.
// Use RemoveAll to stop & remove the containers.
func (Sakila) StartAll(ctx context.Context) error {
	const envarInfo = `export SQ_TEST_SRC__SAKILA_MY56=localhost:33066
export SQ_TEST_SRC__SAKILA_MY57=localhost:33067
export SQ_TEST_SRC__SAKILA_MY8=localhost:33068
export SQ_TEST_SRC__SAKILA_PG9=localhost:54329
export SQ_TEST_SRC__SAKILA_PG10=localhost:54330
export SQ_TEST_SRC__SAKILA_PG11=localhost:54331
export SQ_TEST_SRC__SAKILA_PG12=localhost:54332
export SQ_TEST_SRC__SAKILA_MS17=localhost:14337`

	fmt.Println("Starting all containers...")
	fmt.Printf("'src' the following:\n===\n%s\n===\n\n", envarInfo)

	errGroup, ctx := errgroup.WithContext(ctx)

	for _, containerReq := range requests {
		containerReq := containerReq
		// The user invokes mage sakila:RemoveAll to remove all the containers.
		containerReq.SkipReaper = true

		errGroup.Go(func() error {
			fmt.Printf("Starting container %s for %s\n", containerReq.Name, containerReq.Image)

			gotContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: containerReq,
				Started:          true,
			})

			if err != nil {
				return fmt.Errorf("image %s: %w", containerReq.Image, err)
			}

			containerStarted(gotContainer)

			fmt.Printf("SUCCESS: started container %s for %s\n", containerReq.Name, containerReq.Image)
			return nil
		})
	}

	err := errGroup.Wait()
	if err != nil {
		fmt.Println("ERROR: problem reported trying to start all containers (some or all containers may actually have started)")
		return err
	}

	fmt.Println("SUCCESS: All database containers started")

	return nil
}

// RemoveAll removes all the Sakila docker containers.
func (Sakila) RemoveAll(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Add(len(requests))
	for _, containerReq := range requests {
		containerReq := containerReq

		go func() {
			defer wg.Done()
			// We don't care if there's an error
			_ = execDocker("rm", "-f", containerReq.Name)
		}()
	}

	wg.Wait()
}
