package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/HMasataka/collision/domain/entity"
	"github.com/HMasataka/collision/gen/pb"
	"github.com/HMasataka/errs"
	"github.com/jessevdk/go-flags"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Options struct {
	Host    string `short:"h" long:"host" description:"Server host" default:"127.0.0.1"`
	Port    string `short:"p" long:"port" description:"Server port" default:"31080"`
	Players int    `short:"n" long:"players" description:"Number of players" default:"4"`
}

func getConnection(host, port string) (*grpc.ClientConn, error) {
	address := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	fmt.Println("Connecting to", address)

	return conn, nil
}

func createTicket(ctx context.Context, client pb.FrontendServiceClient, playerID string) (string, *errs.Error) {
	// Create a ticket with search fields for matchmaking
	searchFields := &pb.SearchFields{
		StringArgs: map[string]string{
			"mode": "1vs1",
		},
		Tags: []string{"casual"},
	}

	response, err := client.CreateTicket(ctx, &pb.CreateTicketRequest{
		SearchFields: searchFields,
		Extensions:   fmt.Appendf(nil, `{"player_id": "%s"}`, playerID),
	})
	if err != nil {
		return "", entity.ErrTicketCreateFailed.WithCause(err)
	}

	fmt.Printf("Created ticket %s for player %s\n", response.Id, playerID)
	return response.Id, nil
}

func watchAssignments(ctx context.Context, client pb.FrontendServiceClient, ticketID string, wg *sync.WaitGroup) {
	defer wg.Done()

	stream, err := client.WatchAssignments(ctx, &pb.WatchAssignmentsRequest{
		TicketId: ticketID,
	})
	if err != nil {
		log.Printf("Failed to watch assignments for ticket %s: %v", ticketID, err)
		return
	}

	fmt.Printf("Watching assignments for ticket %s...\n", ticketID)

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			fmt.Printf("Assignment stream ended for ticket %s\n", ticketID)
			return
		}
		if err != nil {
			log.Printf("Error receiving assignment for ticket %s: %v", ticketID, err)
			return
		}

		if response.Assignment != nil {
			fmt.Printf("🎉 MATCH FOUND! Ticket %s assigned to server: %s\n",
				ticketID, response.Assignment.Connection)
			return
		}
	}
}

func main() {
	var opts Options
	parser := flags.NewParser(&opts, flags.Default)
	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	conn, err := getConnection(opts.Host, opts.Port)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := pb.NewFrontendServiceClient(conn)

	players := make([]string, opts.Players)
	for i := 0; i < opts.Players; i++ {
		players[i] = fmt.Sprintf("Player%d", i+1)
	}
	var ticketIDs []string

	fmt.Println("Creating tickets for players...")
	for _, player := range players {
		ticketID, err := createTicket(ctx, client, player)
		if err != nil {
			log.Printf("Failed to create ticket for %s: %v", player, err)
			continue
		}
		ticketIDs = append(ticketIDs, ticketID)

		// Small delay between ticket creation
		time.Sleep(100 * time.Millisecond)
	}

	if len(ticketIDs) == 0 {
		log.Fatal("No tickets were created successfully")
	}

	fmt.Printf("\n✅ Created %d tickets successfully\n", len(ticketIDs))
	fmt.Println("Starting to watch for assignments...")

	// Watch assignments for all tickets concurrently
	var wg sync.WaitGroup
	for _, ticketID := range ticketIDs {
		wg.Add(1)
		go watchAssignments(ctx, client, ticketID, &wg)

		// Small delay to stagger the watch requests
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for all assignment watchers to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("\n🏁 All assignment watching completed")
	case <-ctx.Done():
		fmt.Println("\n⏰ Timeout reached, stopping...")
	}

	// Clean up: delete any remaining tickets
	fmt.Println("\nCleaning up tickets...")
	for _, ticketID := range ticketIDs {
		_, err := client.DeleteTicket(context.Background(), &pb.DeleteTicketRequest{
			TicketId: ticketID,
		})
		if err != nil {
			log.Printf("Failed to delete ticket %s: %v", ticketID, err)
		} else {
			fmt.Printf("Deleted ticket %s\n", ticketID)
		}
	}
}
