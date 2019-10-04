package main

/*
import "context"

// This is a demo handler for the cluster

// Request is your run-of-the-mill request object
type Request struct {
}

// Response is your run-of-the-mill response object
type Response struct {
}

type ServiceRequestObject struct {
}

type ServiceResponseObject struct {
}

type WorkerFunc func(context.Context, Request) Response

// ClusterWorker is a worker object for the request handlers
type ClusterWorker struct {
	workItems chan workItem
}

type workItem struct {
	Context context.Context
	Result  chan Response
	Request Request
	Work    WorkerFunc
}

func (w *ClusterWorker) worker() {
	for item := range w.workItems {
		// wait for the cluster to sort itself out
		w.Cluster.WaitForState(cluster.OK)
		select {
		case <-item.Context.Done():
			// discard item, it's done
			continue
		default:
			if item.Destination == cluster.LocalNode() {
				// process locally
				continue
			}
			// Proxy request to another node
			rpcHost := getRPCHost(item.Destination)
			item.Result <- rpcHost.Work(item.Context, item.Request)
		}
	}
}

// NewClusterWorker creates a new cluster worker.
func NewClusterWorker(workerCount int) *ClusterWorker {
	ret := &ClusterWorker{
		workItems: make(chan workItem),
	}

	for i := 0; i < workerCount; i++ {
		go ret.worker()
	}
	return ret
}

// Enqueue enqueues a worker function
func (w *ClusterWorker) Enqueue(ctx context.Context, req Request, f WorkerFunc) <-chan Response {
	ret := make(chan Response)
	w.workItems <- workItem{
		Context: ctx,
		Result:  ret,
		Request: req,
	}
	return ret
}

// RequestHandler is your handler type
type RequestHandler struct {
	worker ClusterWorker
}

// Handle handles a request
func (rh *RequestHandler) Handle(r ServiceRequestObject) ServiceResponseObject {
	return rh.toServiceResponseObject(<-rh.worker.Enqueue(context.Background(), rh.fromServiceRequestObject(r), rh.workFunc))
}

// These are the required conversion methods to and from the
func (rh *RequestHandler) toServiceResponseObject(r Response) ServiceResponseObject {
	return ServiceResponseObject{}
}

func (rh *RequestHandler) fromServiceRequestObject(r ServiceRequestObject) Request {
	return Request{}
}

func (rh *RequestHandler) workFunc(ctx context.Context, r Request) Response {
	return Response{}
}
*/
