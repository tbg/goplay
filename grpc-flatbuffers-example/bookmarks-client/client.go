package main

import (
	"context"
	"log"
	"os"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/tbg/goplay/grpc-flatbuffers-example/bookmarks"

	"google.golang.org/grpc"
)

type server struct{}

var addr = "0.0.0.0:50051"

func main() {

	if len(os.Args) < 2 {
		log.Fatalln("Insufficient args provided")
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithCodec(flatbuffers.FlatbuffersCodec{}))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := bookmarks.NewBookmarksServiceClient(conn)

	cmd := os.Args[1]

	if cmd == "add" {

		if len(os.Args) < 4 {
			log.Fatalln("Insufficient args provided for add command..")
		}

		b := flatbuffers.NewBuilder(0)
		url := b.CreateString(os.Args[2])
		title := b.CreateString(os.Args[3])

		bookmarks.AddRequestStart(b)
		bookmarks.AddRequestAddURL(b, url)
		bookmarks.AddRequestAddTitle(b, title)
		b.Finish(bookmarks.AddRequestEnd(b))

		_, err = client.Add(context.Background(), b)
		if err != nil {
			log.Fatalf("Retrieve client failed: %v", err)
		}

	} else if cmd == "last-added" {

		b := flatbuffers.NewBuilder(0)
		bookmarks.LastAddedRequestStart(b)
		b.Finish(bookmarks.LastAddedRequestEnd(b))

		out, err := client.LastAdded(context.Background(), b)
		if err != nil {
			log.Fatalf("Retrieve client failed: %v", err)
		}

		log.Println("ID: ", string(out.ID()))
		log.Println("URL: ", string(out.URL()))
		log.Println("Title: ", string(out.Title()))

	}

	log.Println("SENT")

}
