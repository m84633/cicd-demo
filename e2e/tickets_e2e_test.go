package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	compose "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"partivo_tickets/api"
	ticketspb "partivo_tickets/api/tickets"
)

func TestTicketsAdminLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skip e2e in short mode")
	}

	configureDockerDesktop(t)

	composeFile := filepath.Join("resources", "docker-compose.yaml")

	stack, err := compose.NewDockerCompose(composeFile)
	require.NoError(t, err)

	stack.
		WaitForService("mongo", wait.ForListeningPort("27017/tcp")).
		WaitForService("rabbitmq", wait.ForListeningPort("5672/tcp")).
		WaitForService("redis", wait.ForListeningPort("6379/tcp")).
		WaitForService("console", wait.ForListeningPort("8081/tcp"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	require.NoError(t, stack.Up(ctx, compose.Wait(true)))
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer shutdownCancel()
		_ = stack.Down(shutdownCtx, compose.RemoveOrphans(true), compose.RemoveVolumes(true))
	}()

	consoleContainer, err := stack.ServiceContainer(ctx, "console")
	require.NoError(t, err)

	host, err := consoleContainer.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := consoleContainer.MappedPort(ctx, "8081/tcp")
	require.NoError(t, err)

	target := host + ":" + mappedPort.Port()

	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	conn.Connect()

	connCtx, connCancel := context.WithTimeout(ctx, 30*time.Second)
	defer connCancel()

	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			break
		}
		if !conn.WaitForStateChange(connCtx, state) {
			require.FailNow(t, "failed to establish gRPC connection before timeout", "last state: %s", state.String())
		}
	}

	client := ticketspb.NewTicketsAdminServiceClient(conn)

	md := metadata.New(map[string]string{
		"X-User-Id":    primitive.NewObjectID().Hex(),
		"X-User-Name":  "e2e-admin",
		"X-User-Email": "admin@example.com",
	})
	rpcCtx := metadata.NewOutgoingContext(ctx, md)

	eventID := primitive.NewObjectID().Hex()
	session1 := primitive.NewObjectID().Hex()
	session2 := primitive.NewObjectID().Hex()

	addReq := &ticketspb.AddTicketTypeRequest{
		Event:               eventID,
		Name:                "E2E Ticket",
		Quantity:            100,
		Price:               "199.99",
		Desc:                "end-to-end generated ticket",
		Visibility:          true,
		StartShowingOn:      timestamppb.New(time.Now().Add(-1 * time.Hour)),
		StopShowingOn:       timestamppb.New(time.Now().Add(2 * time.Hour)),
		Pick:                1,
		MinQuantityPerOrder: 1,
		MaxQuantityPerOrder: 5,
		DueDuration:         durationpb.New(4 * time.Hour),
		Enable:              true,
		SaleTimeSetting:     "default",
		Sessions: []*ticketspb.AddTicketTypeRequest_Session{
			{
				Session:     session1,
				SaleStartAt: timestamppb.New(time.Now().Add(-2 * time.Hour)),
				SaleEndAt:   timestamppb.New(time.Now().Add(3 * time.Hour)),
			},
			{
				Session:     session2,
				SaleStartAt: timestamppb.New(time.Now().Add(-90 * time.Minute)),
				SaleEndAt:   timestamppb.New(time.Now().Add(4 * time.Hour)),
			},
		},
	}

	addResp, err := client.AddTicketType(rpcCtx, addReq)
	require.NoError(t, err)
	require.Equal(t, "success", addResp.GetStatus())

	addData := addResp.GetData()
	require.NotNil(t, addData)

	var addID api.ID
	require.NoError(t, addData.UnmarshalTo(&addID))
	ticketTypeID := addID.GetId()
	require.NotEmpty(t, ticketTypeID)

	updateReq := &ticketspb.UpdateTicketTypeRequest{
		Ticket:              ticketTypeID,
		Name:                "E2E Ticket Updated",
		Desc:                "updated description",
		Quantity:            120,
		Price:               "249.99",
		Visibility:          true,
		StartShowingOn:      timestamppb.New(time.Now().Add(-30 * time.Minute)),
		StopShowingOn:       timestamppb.New(time.Now().Add(5 * time.Hour)),
		MinQuantityPerOrder: 1,
		MaxQuantityPerOrder: 6,
		DueDuration:         durationpb.New(6 * time.Hour),
		Enable:              true,
		SaleTimeSetting:     "default",
		Sessions: []*ticketspb.UpdateTicketTypeRequest_Session{
			{
				Session:     session1,
				SaleStartAt: timestamppb.New(time.Now().Add(-2 * time.Hour)),
				SaleEndAt:   timestamppb.New(time.Now().Add(3 * time.Hour)),
			},
			{
				Session:     session2,
				SaleStartAt: timestamppb.New(time.Now().Add(-90 * time.Minute)),
				SaleEndAt:   timestamppb.New(time.Now().Add(4 * time.Hour)),
			},
		},
	}

	updateResp, err := client.UpdateTicketType(rpcCtx, updateReq)
	require.NoError(t, err)
	require.Equal(t, "success", updateResp.GetStatus())

	getResp, err := client.GetEventTicketTypes(rpcCtx, &ticketspb.GetEventTicketTypesRequest{Event: eventID})
	require.NoError(t, err)
	require.Equal(t, "success", getResp.GetStatus())

	getData := getResp.GetData()
	require.NotNil(t, getData)

	var list ticketspb.GetEventTicketTypesResponse
	require.NoError(t, getData.UnmarshalTo(&list))
	require.Len(t, list.Data, 1)
	gotTicket := list.Data[0]
	require.Equal(t, ticketTypeID, gotTicket.GetId())
	require.Equal(t, "E2E Ticket Updated", gotTicket.GetName())
	require.Equal(t, "updated description", gotTicket.GetDesc())

	delResp, err := client.DeleteTicketType(rpcCtx, &ticketspb.DeleteTicketTypeRequest{Ticket: ticketTypeID})
	require.NoError(t, err)
	require.Equal(t, "success", delResp.GetStatus())

	afterResp, err := client.GetEventTicketTypes(rpcCtx, &ticketspb.GetEventTicketTypesRequest{Event: eventID})
	require.NoError(t, err)
	require.Equal(t, "success", afterResp.GetStatus())

	afterData := afterResp.GetData()
	require.NotNil(t, afterData)

	var afterList ticketspb.GetEventTicketTypesResponse
	require.NoError(t, afterData.UnmarshalTo(&afterList))
	require.Len(t, afterList.Data, 0)
}

func configureDockerDesktop(t *testing.T) {
	t.Helper()

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	socket := filepath.Join(home, ".docker", "run", "docker.sock")
	if info, err := os.Stat(socket); err == nil && !info.IsDir() {
		t.Setenv("DOCKER_HOST", "unix://"+socket)
		t.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", socket)
	}
}
