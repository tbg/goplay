package main

import (
	"log"
	"net"
	"strconv"

	context "golang.org/x/net/context"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/tbg/goplay/grpc-flatbuffers-example/bookmarks"

	"google.golang.org/grpc"
)

type server struct {
	id        int
	lastTitle string
	lastURL   string
}

var addr = "0.0.0.0:50051"

func (s *server) Add(context context.Context, in *bookmarks.AddRequest) (*flatbuffers.Builder, error) {
	log.Println("Add called...")

	s.id++
	s.lastTitle = string(in.Title())
	s.lastURL = string(in.URL())

	b := flatbuffers.NewBuilder(0)
	bookmarks.AddResponseStart(b)
	b.Finish(bookmarks.AddResponseEnd(b))
	return b, nil
}

func (s *server) LastAdded(context context.Context, in *bookmarks.LastAddedRequest) (*flatbuffers.Builder, error) {
	log.Println("LastAdded called...")

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

func main() {

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	ser := grpc.NewServer(grpc.CustomCodec(flatbuffers.FlatbuffersCodec{}))

	bookmarks.RegisterBookmarksServiceServer(ser, &server{})
	if err := ser.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

}
