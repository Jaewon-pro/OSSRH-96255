package future_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jaewon-pro/go-practice/future"
)

var (
	ctx = context.Background()
)

func ExampleFuture_Await() {
	f := future.New(func() (int, error) {
		return 42, nil
	})

	result := f.Await(ctx)
	if result.Error != nil {
		panic("result.Error is not nil")
	}

	fmt.Println(result.Value)
	// Output: 42
}

func ExampleFuture_AwaitWithTimeout() {
	f := future.New(func() (int, error) {
		return 42, nil
	})

	result := f.AwaitWithTimeout(ctx, 10*time.Millisecond)
	if result.Error != nil {
		panic("result.Error is not nil")
	}

	fmt.Println(result.Value)
	// Output: 42
}

func ExampleFuture_Async() {
	f := future.New(func() (int, error) {
		return 42, nil
	})

	resultChan, _ := f.Async(ctx)
	result := <-resultChan
	if result.Error != nil {
		panic("result.Error is not nil")
	}
	fmt.Println(result.Value)
	// Output: 42
}

func ExampleFuture_Then() {
	f := future.New(func() (int, error) {
		return 42, nil
	})

	f2 := future.Then(f, ctx, func(value int) (string, error) {
		return fmt.Sprintf("value is %d", value), nil
	})

	result := f2.Await(ctx)
	if result.Error != nil {
		panic("result.Error is not nil")
	}

	fmt.Println(result.Value)
	// Output: value is 42
}

func Test_Future_TimeoutExceeded(t *testing.T) {
	f := future.New(func() (int, error) {
		time.Sleep(10 * time.Millisecond)
		return 42, nil
	})

	f2 := future.Then(f, ctx, func(value int) (string, error) {
		return fmt.Sprintf("value is %d", value), nil
	})

	result := f2.AwaitWithTimeout(ctx, time.Millisecond)
	if result.Error != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", result.Error)
	}
}

func Test_Future_AwaitWithTimeout_ctxDone(t *testing.T) {
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	f := future.New(func() (int, error) {
		return 42, nil
	})

	result := f.AwaitWithTimeout(ctx, time.Millisecond)
	if result.Error != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", result.Error)
	}
}

func Test_Future_Error(t *testing.T) {
	err := fmt.Errorf("error")
	f := future.New(func() (int, error) {
		return 0, err
	})

	f2 := future.Then(f, ctx, func(value int) (string, error) {
		return fmt.Sprintf("value is %d", value), nil
	})

	result := f2.Await(ctx)
	if result.Error != err {
		t.Errorf("expected %v, got %v", err, result.Error)
	}
}

func Test_Future_AllOf_Ordered(t *testing.T) {
	f1 := future.New(func() (int, error) {
		return 1, nil
	})
	f2 := future.New(func() (int, error) {
		return 2, nil
	})
	f3 := future.New(func() (int, error) {
		return 3, nil
	})

	fAll := future.AllOf(ctx, f1, f2, f3)

	result := fAll.Await(ctx)
	if result.Error != nil {
		t.Errorf("expected nil, got %v", result.Error)
	}

	if len(result.Value) != 3 {
		t.Errorf("expected 3, got %d", len(result.Value))
	}

	for i, v := range result.Value {
		if v != i+1 {
			t.Errorf("expected %d, got %d", i+1, v)
		}
	}
}

func Test_Future_AllOf_Error(t *testing.T) {
	goodFuture := future.New(func() (int, error) {
		return 1, nil
	})
	errorFuture := future.New(func() (int, error) {
		return 0, fmt.Errorf("something went wrong")
	})

	fAll := future.AllOf(ctx, goodFuture, errorFuture)

	result := fAll.Await(ctx)
	if result.Error == nil {
		t.Error("expected error, got nil")
	}
}

func Test_Future_AllOf_TimeoutExceeded(t *testing.T) {
	f1 := future.New(func() (int, error) {
		time.Sleep(10 * time.Millisecond)
		return 1, nil
	})
	f2 := future.New(func() (int, error) {
		// No sleep
		return 2, nil
	})

	fAll := future.AllOf(ctx, f1, f2)

	result := fAll.AwaitWithTimeout(ctx, time.Millisecond)
	if result.Error != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", result.Error)
	}
}

func Test_Future_AllOf_ctxDone(t *testing.T) {
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	f1 := future.New(func() (int, error) {
		return 1, nil
	})
	f2 := future.New(func() (int, error) {
		return 2, nil
	})

	fAll := future.AllOf(ctx, f1, f2)

	result := fAll.Await(ctx)
	if result.Error != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", result.Error)
	}
}

func Test_Future_AnyOf_ErrorFuture_should_returned(t *testing.T) {
	myErr := fmt.Errorf("something went wrong")
	goodFuture := future.New(func() (int, error) {
		time.Sleep(10 * time.Millisecond)
		return 1, nil
	})
	errorFuture := future.New(func() (int, error) {
		return 0, myErr
	})

	fAny := future.AnyOf(ctx, goodFuture, errorFuture)

	result := fAny.Await(ctx)
	if result.Error != myErr {
		t.Error("expected error, got nil")
	}
}

func Test_Future_AnyOf_TimeoutExceeded(t *testing.T) {
	slowFuture := future.New(func() (int, error) {
		time.Sleep(10 * time.Millisecond)
		return 1, nil
	})
	fastFuture := future.New(func() (int, error) {
		time.Sleep(2 * time.Millisecond)
		return 2, nil
	})

	fAny := future.AnyOf(ctx, slowFuture, fastFuture)

	result := fAny.AwaitWithTimeout(ctx, time.Millisecond)
	if result.Error != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", result.Error)
	}
}

func Test_Future_AnyOf_Fastest_should_returned(t *testing.T) {
	fastest := future.New(func() (string, error) {
		time.Sleep(time.Millisecond)
		return "fastest", nil
	})
	second := future.New(func() (string, error) {
		time.Sleep(5 * time.Millisecond)
		return "second", nil
	})
	latest := future.New(func() (string, error) {
		time.Sleep(10 * time.Millisecond)
		return "latest", nil
	})

	fAny := future.AnyOf(ctx, latest, second, fastest)

	result := fAny.Await(ctx)
	if result.Error != nil {
		t.Errorf("expected nil, got %v", result.Error)
	}

	if result.Value != "fastest" {
		t.Errorf("expected fastest, got %s", result.Value)
	}
}

func Test_Future_AnyOf_ctxDone(t *testing.T) {
	ctx, cancel := context.WithCancel(ctx)
	cancel()

	f1 := future.New(func() (int, error) {
		return 1, nil
	})
	f2 := future.New(func() (int, error) {
		return 2, nil
	})

	fAny := future.AnyOf(ctx, f1, f2)

	result := fAny.Await(ctx)
	if result.Error != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", result.Error)
	}
}
