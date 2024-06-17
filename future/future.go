package future

import (
	"context"
	"time"
)

// Result is a struct that represents the result of a future, containing a value and an error.
type Result[T, E any] struct {
	Value T
	Error E
}

// Future is an interface that represents a value that will be available in the future.
type Future[T, E any] interface {
	Await(context.Context) Result[T, E]
	AwaitWithTimeout(context.Context, time.Duration) Result[T, E]
	Async(context.Context) (<-chan Result[T, E], context.CancelFunc)
}

type channelFuture[T any] struct {
	fn func() (T, error)
}

func (cf *channelFuture[T]) Await(ctx context.Context) Result[T, error] {
	resultCh, _ := cf.Async(ctx)
	return <-resultCh
}

func (cf *channelFuture[T]) AwaitWithTimeout(ctx context.Context, timeout time.Duration) Result[T, error] {
	resultCh, cancel := cf.Async(ctx)

	select {
	case <-ctx.Done():
		return Result[T, error]{Error: ctx.Err()}
	case result := <-resultCh:
		return result
	case <-time.After(timeout):
		cancel()
		return Result[T, error]{Error: context.DeadlineExceeded}
	}
}

func (cf *channelFuture[T]) Async(ctx context.Context) (<-chan Result[T, error], context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	resultCh := make(chan Result[T, error], 1)

	go func() {
		defer close(resultCh)
		select {
		case <-ctx.Done():
			resultCh <- Result[T, error]{Error: ctx.Err()}
		default:
			value, err := cf.fn()
			resultCh <- Result[T, error]{Value: value, Error: err}
		}
	}()

	return resultCh, cancel
}

// New creates a new future from a function that returns a value and an error.
func New[T any](fn func() (T, error)) Future[T, error] {
	return &channelFuture[T]{fn: fn}
}

// Then takes a future, a context, and a function that takes the value of the future and returns a new value.
// It returns a new future that will resolve with the value returned by the function.
func Then[T, U any](f Future[T, error], ctx context.Context, fn func(T) (U, error)) Future[U, error] {
	return New(func() (U, error) {
		result := f.Await(ctx)
		if result.Error != nil {
			var empty U
			return empty, result.Error
		}
		return fn(result.Value)
	})
}

// AllOf takes a context and a slice of futures and returns a new future that will resolve when all the futures resolve.
// If any of the futures return an error, the returned future will return that error with canceling the context.
func AllOf[T any](ctx context.Context, futures ...Future[T, error]) Future[[]T, error] {
	return New(func() ([]T, error) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		resultCh := make(chan struct {
			index  int
			result Result[T, error]
		}, len(futures))
		defer close(resultCh)

		for i, f := range futures {
			go func(i int, f Future[T, error]) {
				rCh, _ := f.Async(ctx)
				select {
				case result := <-rCh:
					resultCh <- struct {
						index  int
						result Result[T, error]
					}{index: i, result: result}
				case <-ctx.Done():
					return
				}
			}(i, f)
		}

		values := make([]T, len(futures))
		for i := 0; i < len(futures); i++ {
			r := <-resultCh
			if r.result.Error != nil {
				cancel()
				return nil, r.result.Error
			}
			values[r.index] = r.result.Value
		}

		return values, nil
	})
}

// AnyOf takes a context and a slice of futures and returns a new future that will resolve when any of the futures resolve.
// If any of the futures return an error, the returned future will return that error with canceling the context.
func AnyOf[T any](ctx context.Context, futures ...Future[T, error]) Future[T, error] {
	return New(func() (T, error) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		resultCh := make(chan Result[T, error])

		for _, f := range futures {
			go func(f Future[T, error]) {
				rCh, _ := f.Async(ctx)
				select {
				case resultCh <- <-rCh:
					cancel()
				case <-ctx.Done():
					return
				}
			}(f)
		}

		result := <-resultCh
		return result.Value, result.Error
	})
}
