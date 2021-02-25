package gearman

import "context"

type Client struct {
	ts *TaskSet
	cm *ClientMisc
}

func NewClient(server []string) *Client {
	return new(Client).Init(server)
}

func (c *Client) Init(server []string) *Client {
	var ds = NewDispatcher(server)

	// set up modules
	c.ts = NewTaskSet().registerResponseHandle(ds)
	c.cm = newClientMisc().registerResponseHandler(ds)

	return c
}

// add task, see TaskOptFuncs for all use case
func (c *Client) AddTask(ctx context.Context, funcName string, data []byte, opt ...TaskOptFunc) (*Task, error) {
	return c.ts.AddTask(ctx, funcName, data, opt...)
}

// get task status of handle default, see TaskStatusFuncs for all use case
func (c *Client) TaskStatus(ctx context.Context, task *Task, opts ...TaskStatusOptFunc) (TaskStatus, error) {
	return c.ts.TaskStatus(ctx, task, opts...)
}

func (c *Client) Echo(ctx context.Context, server string, data []byte) ([]byte, error) {
	return c.cm.Echo(ctx, server, data)
}

func (c *Client) SetConnOption(ctx context.Context, name string) (string, error) {
	return c.cm.SetConnOption(ctx, name)
}

type ClientMisc struct {
	sender *Sender
}

func newClientMisc() *ClientMisc {
	return new(ClientMisc)
}

func (cm *ClientMisc) SetConnOption(ctx context.Context, name string) (string, error) {
	var req = newRequestWithType(PtOptionReq)
	req.SetConnOption(name)

	resp, err := cm.sender.sendAndWaitResp(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.GetConnOption()
}

func (cm *ClientMisc) Echo(ctx context.Context, server string, data []byte) ([]byte, error) {
	var req = newRequestToServerWithType(server, PtEchoReq)
	req.SetData(data)

	resp, err := cm.sender.sendAndWaitResp(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.GetData()
}

func (cm *ClientMisc) registerResponseHandler(ds *Dispatcher) *ClientMisc {
	cm.sender = newSender(ds)

	var handlers = []ResponseTypeHandler{
		{[]PacketType{PtOptionRes}, func(resp *Response) { cm.sender.respCh <- resp }},
		{[]PacketType{PtEchoRes}, func(resp *Response) { cm.sender.respCh <- resp }},
	}

	ds.RegisterResponseHandler(handlers...)

	return cm
}
