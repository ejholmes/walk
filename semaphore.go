package main

// Semaphore is an interface to represent a Semaphore. This is used when
// executing a graph to control the number of concurrent processes running.
type Semaphore interface {
	P()
	V()
}

// NewSemaphore returns a new Semaphore implementation that limits the level of
// concurrency. If concurrency is less than 0, then it returns a semaphore that
// does not limit the amount of concurrency.
func NewSemaphore(concurrency uint) Semaphore {
	if concurrency == 0 {
		return new(unlimitedSemaphore)
	}
	return make(semaphore, concurrency)
}

type semaphore chan struct{}

func (s semaphore) P() {
	s <- struct{}{}
}

func (s semaphore) V() {
	<-s
}

type unlimitedSemaphore struct{}

func (s *unlimitedSemaphore) P() {}
func (s *unlimitedSemaphore) V() {}
