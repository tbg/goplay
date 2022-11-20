package main

import (
	"context"
	"fmt"
	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stretchr/testify/require"
	"github.com/tbg/goplay/grpc-flatbuffers-example/bookmarks"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"log"
	"net"
	"strconv"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	const addr = "127.0.0.1:0"
	lis, err := net.Listen("tcp", addr)
	require.NoError(t, err)
	defer lis.Close()

	encoding.RegisterCodec(flatbuffers.FlatbuffersCodec{})
	srv := grpc.NewServer()
	bookmarks.RegisterBookmarksServiceServer(srv, &testServer{})

	go func() {
		_ = srv.Serve(lis)
	}()

	creds := grpc.WithTransportCredentials(insecure.NewCredentials())
	codec := grpc.WithDefaultCallOptions(grpc.ForceCodec(flatbuffers.FlatbuffersCodec{}))
	conn, err := grpc.Dial(lis.Addr().String(), creds, codec)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := bookmarks.NewBookmarksServiceClient(conn)
	for i := 0; i < 10; i++ {
		{
			b := flatbuffers.NewBuilder(0)
			url := b.CreateString(fmt.Sprintf("https://server/link/%d", i+1))
			title := b.CreateString(fmt.Sprintf("my bookmark #%d", i+1))

			bookmarks.AddRequestStart(b)
			bookmarks.AddRequestAddURL(b, url)
			bookmarks.AddRequestAddTitle(b, title)
			b.Finish(bookmarks.AddRequestEnd(b))
			_, err := c.Add(ctx, b)
			require.NoError(t, err)
		}

		{
			b := flatbuffers.NewBuilder(0)
			bookmarks.LastAddedRequestStart(b)
			b.Finish(bookmarks.LastAddedRequestEnd(b))
			resp, err := c.LastAdded(ctx, b)
			require.NoError(t, err)
			t.Logf("id %s: title %s, url %s", resp.ID(), resp.Title(), resp.URL())
			t.Logf("table: %q", resp.Table())
		}
	}
}

type testServer struct {
	id        int
	lastTitle string
	lastURL   string
}

func (s *testServer) Add(context context.Context, in *bookmarks.AddRequest) (*flatbuffers.Builder, error) {
	s.id++
	s.lastTitle = string(in.Title())
	s.lastURL = string(in.URL())

	b := flatbuffers.NewBuilder(0)
	bookmarks.AddResponseStart(b)
	b.Finish(bookmarks.AddResponseEnd(b))
	return b, nil
}

func (s *testServer) LastAdded(context context.Context, in *bookmarks.LastAddedRequest) (*flatbuffers.Builder, error) {
	b := flatbuffers.NewBuilder(0)
	id := b.CreateString(strconv.Itoa(s.id))
	title := b.CreateString(s.lastTitle)
	url := b.CreateString(s.lastURL)

	bookmarks.LastAddedResponseStart(b)
	bookmarks.LastAddedResponseAddID(b, id)
	bookmarks.LastAddedResponseAddTitle(b, title)
	bookmarks.LastAddedResponseAddURL(b, url)
	b.Finish(bookmarks.LastAddedResponseEnd(b))
	return b, nil
}
