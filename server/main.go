package main

import (
	"log"
	"math"
	"net"
	"time"

	simpb "github.com/ramsesyok/grpc-store-stub/gen/simulation"
	"google.golang.org/grpc"
)

type simServer struct {
	simpb.UnimplementedSimulationServiceServer
}

type objectState struct {
	id        int32
	x, y, z   float64
	direction float64
	speed     float64
}

func reflect(pos, min, max, dir float64) (float64, float64) {
	if pos < min {
		pos = min + (min - pos)
		dir = math.Pi - dir
	} else if pos > max {
		pos = max - (pos - max)
		dir = math.Pi - dir
	}
	return pos, dir
}

func (s *simServer) RunSimulation(req *simpb.SimulationRequest, stream simpb.SimulationService_RunSimulationServer) error {
	interval := req.GetInterval()
	if interval <= 0 {
		interval = 1.0
	}
	bulkSize := int(req.GetBulkSize())
	if bulkSize <= 0 {
		bulkSize = 1
	}
	waitDur := time.Duration(float64(time.Second) * req.GetWait())

	area := req.GetArea()
	rangeReq := req.GetRange()
	start := rangeReq.GetStart()
	end := rangeReq.GetEnd()

	// 初期状態を設定
	states := make([]objectState, 0, len(req.GetObjects()))
	for _, o := range req.GetObjects() {
		states = append(states, objectState{
			id:        o.GetId(),
			x:         o.GetX(),
			y:         o.GetY(),
			z:         o.GetZ(),
			direction: o.GetDirection(),
			speed:     o.GetSpeed(),
		})
	}

	items := make([]*simpb.SimItem, 0, bulkSize)
	chunkStart := start

	for t := start; t < end; t += interval {
		// 各オブジェクトを1ステップ更新
		attrs := make([]*simpb.SimAttribute, 0, len(states))
		for i := range states {
			st := &states[i]
			rad := st.direction * math.Pi / 180.0
			st.x += st.speed * math.Cos(rad) * interval
			st.y += st.speed * math.Sin(rad) * interval

			// area反射
			st.x, st.direction = reflect(st.x, area.GetXMin(), area.GetXMax(), st.direction)
			st.y, st.direction = reflect(st.y, area.GetYMin(), area.GetYMax(), st.direction)

			attrs = append(attrs, &simpb.SimAttribute{
				Id:        st.id,
				X:         st.x,
				Y:         st.y,
				Z:         st.z,
				Direction: st.direction,
				Speed:     st.speed,
			})
		}

		items = append(items, &simpb.SimItem{
			Timestamp:  t + interval,
			Attributes: attrs,
			Events:     []*simpb.SimEvent{},
		})

		// bulkSize 分たまったら送信
		if len(items) >= bulkSize {
			chunkEnd := t + interval
			isFinal := chunkEnd >= end
			resp := &simpb.SimulationResponse{
				Items:     items,
				ItemCount: int32(len(items)),
				Range:     &simpb.Range{Start: chunkStart, End: chunkEnd},
				IsFinal:   isFinal,
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
			if waitDur > 0 {
				time.Sleep(waitDur)
			}
			chunkStart = chunkEnd
			items = make([]*simpb.SimItem, 0, bulkSize)
			if isFinal {
				return nil
			}
		}
	}

	// 残りを送信
	if len(items) > 0 {
		resp := &simpb.SimulationResponse{
			Items:     items,
			ItemCount: int32(len(items)),
			Range:     &simpb.Range{Start: chunkStart, End: end},
			IsFinal:   true,
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	simpb.RegisterSimulationServiceServer(s, &simServer{})
	log.Println("server listening on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
