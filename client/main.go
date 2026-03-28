package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	simpb "github.com/ramsesyok/grpc-store-stub/gen/simulation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  client run --input <file> [--server <addr>]")
		fmt.Fprintln(os.Stderr, "  client gen --output <file> [--objects <n>] [--width <w>] [--height <h>]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runCmd(os.Args[2:])
	case "gen":
		genCmd(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// --- run ---

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	input := fs.String("input", "request.json", "path to request JSON file")
	server := fs.String("server", "localhost:50051", "server address")
	fs.Parse(args)

	data, err := os.ReadFile(*input)
	if err != nil {
		log.Fatalf("failed to read %s: %v", *input, err)
	}

	req := &simpb.SimulationRequest{}
	if err := protojson.Unmarshal(data, req); err != nil {
		log.Fatalf("failed to parse request JSON: %v", err)
	}

	conn, err := grpc.NewClient(*server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	c := simpb.NewSimulationServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stream, err := c.RunSimulation(ctx, req)
	if err != nil {
		log.Fatalf("RunSimulation failed: %v", err)
	}

	chunkNum := 0
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("stream error: %v", err)
		}
		chunkNum++
		log.Printf("[chunk %d] itemCount=%d range=[%.2f, %.2f] isFinal=%v",
			chunkNum,
			resp.GetItemCount(),
			resp.GetRange().GetStart(),
			resp.GetRange().GetEnd(),
			resp.GetIsFinal(),
		)
		if resp.GetIsFinal() {
			break
		}
	}
	log.Printf("done. total chunks: %d", chunkNum)
}

// --- gen ---

func genCmd(args []string) {
	fs := flag.NewFlagSet("gen", flag.ExitOnError)
	output     := fs.String("output",      "request.json", "output file path")
	objects    := fs.Int("objects",        1,              "number of objects")
	width      := fs.Float64("width",      100.0,          "area width  (x: 0 to width)")
	height     := fs.Float64("height",     100.0,          "area height (y: 0 to height)")
	rangeStart := fs.Float64("range-start", 0.0,           "simulation range start (seconds)")
	rangeEnd   := fs.Float64("range-end",   1000.0,        "simulation range end   (seconds)")
	interval   := fs.Float64("interval",    1.0,           "simulation step interval (seconds)")
	bulkSize   := fs.Int("bulk-size",       100,           "number of items per response chunk")
	wait       := fs.Float64("wait",        0.5,           "sleep between sends (seconds)")
	fs.Parse(args)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	objs := make([]*simpb.SimObject, *objects)
	for i := range objs {
		objs[i] = &simpb.SimObject{
			Id:        int32(i),
			X:         rng.Float64() * *width,
			Y:         rng.Float64() * *height,
			Z:         rng.Float64() * *height,
			Direction: rng.Float64() * 360.0,
			Speed:     5.0,
		}
	}

	req := &simpb.SimulationRequest{
		Objects: objs,
		Area: &simpb.Area{
			XMin: 0, XMax: *width,
			YMin: 0, YMax: *height,
		},
		Range:    &simpb.Range{Start: *rangeStart, End: *rangeEnd},
		Interval: *interval,
		BulkSize: int32(*bulkSize),
		Wait:     *wait,
	}

	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(req)
	if err != nil {
		log.Fatalf("failed to marshal request: %v", err)
	}

	if err := os.WriteFile(*output, data, 0644); err != nil {
		log.Fatalf("failed to write %s: %v", *output, err)
	}
	log.Printf("generated %s (%d objects, area %.0fx%.0f)", *output, *objects, *width, *height)
}
