package main

import (
	"context"
	"log"
	"time"

	"github.com/orionbetasp/go-gearman"
)

func main() {
	var server = []string{"localhost:4730", "localhost:4731"}

	var client = gearman.NewClient(server)

	// echo for test
	for _, s := range server {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		ret, err := client.Echo(ctx, s, []byte("hello"))
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("echo, %s return %s", s, string(ret))
	}

	var funcName = "test"

	ctxBg, cancelBg := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelBg()
	// do background task
	_, _ = client.AddTask(
		ctxBg,
		// function name
		funcName,
		// data sent to worker
		[]byte("background"),
		// gearman.TaskOptHigh(), set task priority high
		// gearman.TaskOptHighBackground(), set background task high priority
		// gearman.TaskOptLow(), set task priority low
		// gearman.TaskOptLowBackground(),	set background task low priority
		// gearman.TaskOptNormal(), set task priority normal
		// gearman.TaskOptNormalBackground(), set background task normal priority
		gearman.TaskOptNormalBackground(),
	)

	for i := 0; i < 10; i++ {
		i := i
		go func() {
			log.Printf("run non-background task %d", i)

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			// do non-background task, block until task complete
			task, err := client.AddTask(
				ctx,
				funcName,
				[]byte("non-background"),
				// handler for task data update
				gearman.TaskOptOnData(func(resp *gearman.Response) {
					data, _ := resp.GetData()
					log.Printf("task update, data returned from worker is '%s'", string(data))
				}),
			)
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("task send to %s", task.Remote())

			// wait for complete
			data, err := task.Wait(ctx)
			log.Printf("task finished, returned value '%s'", string(data))
		}()
	}

}
