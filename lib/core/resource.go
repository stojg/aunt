package core

import "sync"

type Resource interface {
	Name() string
	Headers() []string
	Row() []string
	Display() bool
	SetMetrics()
}

// Merge merges (zip) a slice of resource channels into one single out channel
func Merge(in []chan Resource) chan Resource {
	var wg sync.WaitGroup
	out := make(chan Resource)
	output := func(c chan Resource) {
		for resource := range c {
			out <- resource
		}
		wg.Done()
	}
	wg.Add(len(in))
	for _, c := range in {
		go output(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// Metrics tells the resources to update their metrics
func Metrics(in chan Resource) chan Resource {
	out := make(chan Resource)
	go func() {
		for resource := range in {
			resource.SetMetrics()
			out <- resource
		}
		close(out)
	}()
	return out
}

func Filter(in chan Resource) chan Resource {
	out := make(chan Resource)
	go func() {
		for resource := range in {
			if resource.Display() {
				out <- resource
			}
		}
		close(out)
	}()
	return out
}
