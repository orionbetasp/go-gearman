package gearman

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	defaultTaskPacketType = PtSubmitJob
)

type Task struct {
	Type     PacketType
	FuncName string

	Handle string // set by response

	OnData ResponseHandler

	reqOpt []ReqOptFunc

	// task response chan
	respCh chan *Response

	peer *TransportPeer
	ts   *TaskSet
}

type ReqOptFunc func(*Request)
type TaskOptFunc func(*Task)
type TaskStatusOptFunc func(*Request)

func (t *Task) NonBackground() bool {
	return t.Type == PtSubmitJob ||
		t.Type == PtSubmitJobHigh ||
		t.Type == PtSubmitJobLow
}

func (t *Task) Remote() string {
	return t.peer.Remote
}

// non-background job wait for complete
func (t *Task) Wait(ctx context.Context) ([]byte, error) {
	if !t.NonBackground() {
		return nil, errors.New("no wait for background task")
	}

	// remove from task set
	defer t.ts.removeTask(t.peer.Remote, t.Handle)

	var wg sync.WaitGroup
	for {
		select {
		case resp := <-t.respCh:
			switch resp.Type {
			case PtWorkComplete:
				return resp.GetData()
			case PtWorkData:
				wg.Add(1)
				go func() {
					wg.Done()
					if t.OnData != nil {
						t.OnData(resp)
					}
				}()
			case PtWorkFail:
				return nil, TaskFailError
			}
		case <-ctx.Done():
			return nil, errors.New("wait non-background job complete timeout")
		case <-t.peer.Closed():
			return nil, NetworkError
		}
	}

	return nil, nil
}

type TaskSet struct {
	tasks map[string]map[string]*Task

	trMux sync.RWMutex // lock for task remove

	tcSender *Sender
	tsSender *Sender
}

func NewTaskSet() *TaskSet {
	return &TaskSet{
		tasks: make(map[string]map[string]*Task),
	}
}

// add task, see TaskOptFunc for all use case
func (t *TaskSet) AddTask(ctx context.Context, funcName string, data []byte, opts ...TaskOptFunc) (*Task, error) {
	var task = &Task{
		FuncName: funcName,
		Type:     defaultTaskPacketType,
		respCh:   make(chan *Response),
	}

	// set options
	for _, opt := range opts {
		opt(task)
	}

	var req = new(Request)

	// set request args
	req.SetType(task.Type)
	req.SetFuncName(task.FuncName)
	req.SetData(data)

	// args option
	for _, opt := range task.reqOpt {
		opt(req)
	}

	resp, err := t.tcSender.sendAndWaitResp(ctx, req)
	if err != nil {
		return nil, err
	}

	// set handle
	handle, err := resp.GetHandle()
	if err != nil {
		return nil, err
	}

	Log.Printf("task created, handle %s, server %s", handle, resp.peer.Remote)

	// set peer
	task.peer = resp.peer

	task.Handle = handle

	if task.NonBackground() {
		t.setTask(task.peer.Remote, handle, task)
		task.ts = t
	}

	return task, err
}

// check the status of task, see TaskStatusOptFuncs for all use case
func (t *TaskSet) TaskStatus(ctx context.Context, task *Task, opts ...TaskStatusOptFunc) (ts TaskStatus, err error) {
	var req = new(Request)
	req.Server = task.peer.Remote

	for _, opt := range opts {
		opt(req)
	}

	resp, err := t.tsSender.sendAndWaitResp(ctx, req)
	if err != nil {
		return ts, err
	}

	// TODO: debug int parse
	ts.Known, _ = resp.GetStatusKnow()
	ts.Running, _ = resp.GetStatusRunning()
	ts.Numerator, _ = resp.GetPercentNumerator()
	ts.Denominator, _ = resp.GetPercentDenominator()
	ts.WaitingClient, _ = resp.GetWaitingClientNum()

	return ts, nil
}

func (t *TaskSet) registerResponseHandle(ds *Dispatcher) *TaskSet {
	t.tcSender = newSender(ds)
	t.tsSender = newSender(ds)

	var handlers = []ResponseTypeHandler{
		{
			[]PacketType{PtJobCreated}, func(resp *Response) { t.tcSender.respCh <- resp },
		},
		{
			[]PacketType{PtStatusRes, PtStatusResUnique}, func(resp *Response) { t.tsSender.respCh <- resp },
		},
		{
			[]PacketType{PtWorkData, PtWorkStatus, PtWorkComplete, PtWorkWarning, PtWorkFail, PtWorkException},
			t.workResponseHandle,
		},
	}

	ds.RegisterResponseHandler(handlers...)

	return t
}

func (t *TaskSet) workResponseHandle(resp *Response) {
	handle, _ := resp.GetHandle()

	Log.Printf("taskset get response, handle %s, remote %s", handle, resp.peer.Remote)

	startTime := time.Now()
	endTime := startTime.Add(DefaultSendTimeout)
	for {
		if time.Now().After(endTime) {
			return
		}
		if task, ok := t.getTask(resp.peer.Remote, handle); ok {
			go func() { task.respCh <- resp }()
		}
		time.Sleep(DefaultSendTimeout / 10)
	}
}

func (t *TaskSet) setTask(server, handle string, task *Task) {
	t.trMux.Lock()
	defer t.trMux.Unlock()
	if _, ok := t.tasks[server]; !ok {
		t.tasks[server] = make(map[string]*Task)
	}

	t.tasks[server][handle] = task
}

func (t *TaskSet) getTask(server, handle string) (*Task, bool) {
	t.trMux.RLock()
	defer t.trMux.RUnlock()
	if ts, ok := t.tasks[server]; ok {
		task, ok := ts[handle]
		return task, ok
	}

	return nil, false
}

func (t *TaskSet) removeTask(server, handle string) {
	t.trMux.Lock()
	defer t.trMux.Unlock()
	if ts, ok := t.tasks[server]; ok {
		delete(ts, handle)
	}
}

type TaskStatus struct {
	Known         bool   // know status
	Running       bool   // running status
	Numerator     uint32 // numerator of percent
	Denominator   uint32 // denominator of percent
	WaitingClient uint32 // count of waiting clients
}

// option func for task type
func taskTypeOpt(tp PacketType) TaskOptFunc {
	return func(task *Task) { task.Type = tp }
}

// set normal priority async task
func TaskOptNormal() TaskOptFunc { return taskTypeOpt(PtSubmitJob) }

// set normal priority background task
func TaskOptNormalBackground() TaskOptFunc { return taskTypeOpt(PtSubmitJobBg) }

// set high priority async task
func TaskOptHigh() TaskOptFunc { return taskTypeOpt(PtSubmitJobHigh) }

// set high priority background task
func TaskOptHighBackground() TaskOptFunc { return taskTypeOpt(PtSubmitJobHighBg) }

// set low priority async task
func TaskOptLow() TaskOptFunc { return taskTypeOpt(PtSubmitJobLow) }

// set low priority background task
func TaskOptLowBackground() TaskOptFunc { return taskTypeOpt(PtSubmitJobLowBg) }

// set async task data update callback
func TaskOptOnData(handler ResponseHandler) TaskOptFunc {
	return func(t *Task) { t.OnData = handler }
}

// set task unique id
func TaskOptUniqueId(id string) TaskOptFunc {
	return func(t *Task) {
		t.reqOpt = append(t.reqOpt, func(req *Request) { req.SetUniqueId(id) })
	}
}

// set task reducer
func TaskOptReducer(name string) TaskOptFunc {
	return func(t *Task) {
		t.reqOpt = append(t.reqOpt, func(req *Request) { req.SetReducer(name) })
	}
}

// set task schedule
func TaskOptSchedule(sched time.Time) TaskOptFunc {
	return func(t *Task) {
		t.reqOpt = append(t.reqOpt, func(req *Request) { req.SetSchedule(sched) })
	}
}

// set task epoch time
func TaskOptEpoch(epoch int64) TaskOptFunc {
	return func(t *Task) {
		t.reqOpt = append(t.reqOpt, func(req *Request) { req.SetEpoch(epoch) })
	}
}

// set connection option
func TaskOptConnOption(name string) TaskOptFunc {
	return func(t *Task) {
		t.reqOpt = append(t.reqOpt, func(req *Request) { req.SetConnOption(name) })
	}
}

// set timeout of task creation
func TaskOptCreationTimeout(d time.Duration) TaskOptFunc {
	return func(t *Task) {
		t.reqOpt = append(t.reqOpt, func(req *Request) { req.Timeout = time.After(d) })
	}
}

// set unique id of task for task status retrieve
func TaskOptStatusUniqueId(id string) TaskStatusOptFunc {
	return func(req *Request) {
		req.SetType(PtGetStatusUnique)
		req.SetUniqueId(id)
	}
}

// set task handle for task status retrieve
func TaskOptStatusHandle(handle string) TaskStatusOptFunc {
	return func(req *Request) {
		req.SetType(PtGetStatus)
		req.SetHandle(handle)
	}
}
