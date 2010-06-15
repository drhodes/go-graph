package graph

import (
	. "exp/iterable"
	"math"

	"github.com/StepLg/go-erx/src/erx"
)


type ConnectionWeightFunc func(head, tail VertexId) float64

type StopFunc func(node VertexId, sumWeight float64) bool

func SimpleWeightFunc(head, tail VertexId) float64 {
	return float64(1.0)
}

type AllNeighboursExtractor interface {
	GetAllNeighbours(node VertexId) VertexesIterable
}

type allDirectedNeighboursExtractor struct {
	dgraph DirectedGraphArcsReader
}

func (e *allDirectedNeighboursExtractor) GetAllNeighbours(node VertexId) VertexesIterable {
	return e.dgraph.GetAccessors(node)
}

func NewDirectedNeighboursExtractor(gr DirectedGraphArcsReader) AllNeighboursExtractor {
	return AllNeighboursExtractor(&allDirectedNeighboursExtractor{dgraph:gr})
}

type allUndirectedNeighboursExtractor struct {
	ugraph UndirectedGraphEdgesReader
}

func (e *allUndirectedNeighboursExtractor) GetAllNeighbours(node VertexId) VertexesIterable {
	return e.ugraph.GetNeighbours(node)
}

func NewUndirectedNeighboursExtractor(gr UndirectedGraphEdgesReader) AllNeighboursExtractor {
	return AllNeighboursExtractor(&allUndirectedNeighboursExtractor{ugraph:gr})
}

type allMixedNeighboursExtractor struct {
	mgraph MixedGraphConnectionsReader
}

func (e *allMixedNeighboursExtractor) GetAllNeighbours(node VertexId) VertexesIterable {
	return GenericToVertexesIter(Chain(&[...]Iterable{
		VertexesToGenericIter(e.mgraph.GetAccessors(node)), 
		VertexesToGenericIter(e.mgraph.GetNeighbours(node)),
	}))
}

func NewMixedNeighboursExtractor(gr MixedGraphConnectionsReader) AllNeighboursExtractor {
	return AllNeighboursExtractor(&allMixedNeighboursExtractor{mgraph:gr})
}

// Generic check path algorithm for all graph types
// 
// Checking path between from and to nodes, using getNeighbours function
// to figure out connected nodes on each step of algorithm.
// 
// stopFunc is used to cut bad paths using user-defined criteria
// 
// weightFunction calculates total path weight
// 
// As a result CheckPathDijkstra returns total weight of path, if it exists.
func CheckPathDijkstra(neighboursExtractor AllNeighboursExtractor, from, to VertexId, stopFunc StopFunc, weightFunction ConnectionWeightFunc) (float64, bool) {
	defer func() {
		if e:=recover(); e!=nil {
			err := erx.NewSequent("Check path graph with Dijkstra algorithm", e)
			err.AddV("from", from)
			err.AddV("to", to)
			panic(err)
		}
	}()
	
	if from==to {
		return 0.0, true
	}
	
	q := newPriorityQueueSimple(10)
	q.Add(from, 0.0)
	
	for !q.Empty() {
		curNode, curWeight := q.Next()
		curWeight = -curWeight // because we inverse weight in priority queue
	
		for nextNode := range neighboursExtractor.GetAllNeighbours(curNode).VertexesIter() {
			arcWeight := weightFunction(curNode, nextNode)
			if arcWeight < 0 {
				err := erx.NewError("Negative weight detected")
				err.AddV("head", curNode)
				err.AddV("tail", nextNode)
				err.AddV("weight", arcWeight)
				panic(err)
			}
			nextWeight := curWeight + arcWeight
			if nextNode==to {
				return nextWeight, true
			}
			if stopFunc==nil || !stopFunc(nextNode, nextWeight) {
				q.Add(nextNode, -nextWeight)
			}
		}
	}
	
	return -1.0, false
}

type CheckDirectedPath func(gr DirectedGraphArcsReader, from, to VertexId, stopFunc StopFunc, weightFunction ConnectionWeightFunc) bool

func CheckDirectedPathDijkstra(gr DirectedGraphArcsReader, from, to VertexId, stopFunc StopFunc, weightFunction ConnectionWeightFunc) bool {
	_, pathExists := CheckPathDijkstra(NewDirectedNeighboursExtractor(gr), from, to, stopFunc, weightFunction)
	return pathExists
}

type CheckUndirectedPath func(gr UndirectedGraphEdgesReader, from, to VertexId, stopFunc StopFunc, weightFunction ConnectionWeightFunc) bool

func CheckUndirectedPathDijkstra(gr UndirectedGraphEdgesReader, from, to VertexId, stopFunc StopFunc, weightFunction ConnectionWeightFunc) bool {
	_, pathExists := CheckPathDijkstra(NewUndirectedNeighboursExtractor(gr), from, to, stopFunc, weightFunction)
	return pathExists
}

type CheckMixedPath func(gr MixedGraphConnectionsReader, from, to VertexId, stopFunc StopFunc, weightFunction ConnectionWeightFunc) bool

func CheckMixedPathDijkstra(gr MixedGraphConnectionsReader, from, to VertexId, stopFunc StopFunc, weightFunction ConnectionWeightFunc) bool {
	_, pathExists := CheckPathDijkstra(NewMixedNeighboursExtractor(gr), from, to, stopFunc, weightFunction)
	return pathExists
}

// Get all paths from one node to another
//
// This algorithms doesn't take any loops into paths.
func GetAllPaths(neighboursExtractor AllNeighboursExtractor, from, to VertexId) <-chan []VertexId {
	curPath := make([]VertexId, 10)
	nodesStatus := make(map[VertexId]bool)
	ch := make(chan []VertexId)
	go getAllPaths_helper(neighboursExtractor, from, to, curPath, 0, nodesStatus, ch, true)
	return ch
}

func getAllPaths_helper(neighboursExtractor AllNeighboursExtractor, from, to VertexId, curPath []VertexId, pathPos int, nodesStatus map[VertexId]bool, ch chan []VertexId, closeChannel bool) {
	if _, ok := nodesStatus[from]; ok {
		return
	}
	if pathPos==len(curPath) {
		// reallocate curPath slice to add new elements
		tmp := make([]VertexId, 2*pathPos)
		copy(tmp, curPath)
		curPath = tmp
	}
	
	curPath[pathPos] = from

	if from==to { 
		if pathPos>0 {
			pathCopy := make([]VertexId, pathPos+1)
			copy(pathCopy, curPath[0:pathPos+1])
			ch <- pathCopy
		}
		return
	}
	nodesStatus[from] = true
	
	for nextNode := range neighboursExtractor.GetAllNeighbours(from).VertexesIter() {
		getAllPaths_helper(neighboursExtractor, nextNode, to, curPath, pathPos+1, nodesStatus, ch, false)
	}
	
	nodesStatus[from] = false, false
	
	if closeChannel {
		close(ch)
	}
	return
}

func GetAllDirectedPaths(gr DirectedGraphArcsReader, from, to VertexId) <-chan []VertexId {
	return GetAllPaths(NewDirectedNeighboursExtractor(gr), from, to)
}

func GetAllUndirectedPaths(gr UndirectedGraphEdgesReader, from, to VertexId) <-chan []VertexId {
	return GetAllPaths(NewUndirectedNeighboursExtractor(gr), from, to)
}

func GetAllMixedPaths(gr MixedGraphConnectionsReader, from, to VertexId) <-chan []VertexId {
	return GetAllPaths(NewMixedNeighboursExtractor(gr), from, to)
}

// Compute single-source shortest paths with Bellman-Ford algorithm
//
// Returs map, contains all nodes from graph. If there is no path from source to node in map
// then value for this node is math.MaxFloat64
//
// Returns nil if there are negative cycles. 
func BellmanFordSingleSource(gr DirectedGraphReader, source VertexId, weight ConnectionWeightFunc) map[VertexId]float64 {
	marks := make(map[VertexId]float64)
	for node := range gr.VertexesIter() {
		marks[node] = math.MaxFloat64
	}
	
	marks[source] = 0
	
	nodesCnt := gr.VertexesCnt()
	for i:=0; i<nodesCnt; i++ {
		for conn := range gr.ArcsIter() {
			possibleWeight := marks[conn.Tail] + weight(conn.Tail, conn.Head)
			if marks[conn.Head] > possibleWeight {
				marks[conn.Head] = possibleWeight
			}
		}
	}
	
	for conn := range gr.ArcsIter() {
		if marks[conn.Head] > marks[conn.Tail] + weight(conn.Tail, conn.Head) {
			return nil
		}
	}
	
	return marks
}
