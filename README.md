This is full implemented [Gearman](http://gearman.org/news/) API for Golang
fork from github.com/zhanghjster/go-gearman

## Install

~~~go
go get github.com/orionbetasp/go-gearman
~~~

## Use
see examples folder

### Client API

#### func(*Client) AddTask()

~~~go
func (*Client) AddTask(ctx context.Context, funcName string, data []byte, opt ...TaskOptFunc) (*Task, error)
~~~

create new task, 'funcName' is the function name of worker registered, 'data' is the opaque data worker can get. 

if the error is 'NetworkError', you should retrive the task status by TaskStatus() API

'opt' set other option, see the list beblow

* TaskOptNormal()

  set task non-background with normal priority, it is default for every task

* TaskOptNormalBackground()

  task background with normal priority

* TaskOptLow()

  set task non-background with low priority

* TaskOptLowBackground()

  set task background with low priority

* TaskOptHigh()

  set task non-background with high priority

* TaskOptHighBackground()

  set task background with high priority

* TaskOptOnData(handler  ResponseHandler)

  set callback when there is data sent from worker by job.Update() api

* TaskOptUniqueId(id string)

  set task unique id

* TaskOptReducer(name string)

  set task reducer name

* TaskCreationTimeout(d time.Duration)

  set timeout of task creation

* TaskStatus(task *Task, opts …TaskStatusOptFunc) (TaskResult, error)

  retrieve the task status, 'opts' set the options, see the list below

* TaskOptStatusUniqueId(id string)

  set task unique id 

* TaskOptStatusHandle(id string)

  set task handle of the task 

#### Task API

##### func(*Task) Wait() 

~~~go
func (*Task)Wait(ctx context.Context) (data []byte, error)
~~~

non-background task wait for complete, 'data' is the the opaqua data worker send back when complete the job

##### func (*Task) Remote() string

address of server task assigned to 

##  Worker
see examples folder

### Worker API

#### func(*Worker) MaxParallelJobs()

~~~go
func (*Worker)MaxParallelJobs(n int)
~~~

set max parallel jobs worker can handle, not this limitation default

#### func (*Worker)RegisterFunction()

~~~go
func (*Worker) RegisterFunction(funcName string, handle JobHandle, opt WorkerOptFunc) error
~~~

Do function register and unregister, 'funcName' is the function worker will handle, 'handle' is the processor of job. 'opt' set other options, see the list blow

* WorkerOptCanDo()

  register the handle of 'funcName'

* WorkerOptCanDoTimeout(t time.Duration)

  same as WorkerOptCanDo() but with a timeout value, after the timeout server will set the job failed and notify to the client

* WorkerOptCanotDo()

  unregister the handle of 'funcName'

#### func (*Worker) Work()

~~~go
func (*Worker) Work(ctx context.Context) 
~~~

start grab the jobs from server and process

#### type JobHandle 

~~~go
type JobHandle func(job *Job) (data []byte, err error)
~~~

handler of job worker set when register the function for processing the job client submitted to server. send backed the 'data' to client if there is, 'err' indicate job failed

### Job API

#### func (*Job) Update()

~~~go
func (*Job) Update(opt JobOptFunc) error 
~~~

upate job status or send data to client during job running. 'opt' set the options, see the list below

##### JobOptStatus(n, d uint32)

send job complete precent numerator and denominator to client

##### JobOptData(data []byte)

send data to client during runnning

#### func (*Job) Data() 

~~~go
func (*Job) Data() []byte
~~~

return the opaque data client send to worker

## Admin
see examples folder

### func (*Admin) Do()

~~~go
func (*Admin) Do(ctx context.Context, server string, opt AdmOptFunc) ([]string, error)
~~~

send admin command to 'server'. plain text line returned

command 'version' 'workers' 'status' 'shutdown' 'shutdown' 'maxqueue' supported, set by 'opt' functions, see the list blow

* AdmOptVersion()

  show the version of server

* AdmOptWorkers()

  show the worker list of server, returned text line formatter is 

~~~
FD IP-ADDRESS CLIENT-ID : FUNCTION ...
~~~

* AdmOptStatus()

  show the registered functions of server, returned text line formateter is

~~~
FUNCTION\tTOTAL\tRUNNING\tAVAILABLE_WORKERS
~~~

* AdmOptMaxQueueAll(funcName string, n int)

  set command 'maxqueue', set  max queue size for a function for all priority

* AdmOptMaxQueueThreePriority(funcName string, high, normal, low int)

  set max queue size for a function for three priority

* AdmOptShutdown()

  shutdown the server

* AdmOptShutdownGraceful()

  shutdown the server graceful

## License

MIT



