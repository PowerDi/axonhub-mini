package pipeline

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type disabledRetryBudgetOutbound struct {
	mockOutboundWithRetryLimit
	hammer bool
}

func (m *disabledRetryBudgetOutbound) OverridesSameChannelLimit() bool      { return m.hammer }
func (m *disabledRetryBudgetOutbound) SameChannelRetryDelay() time.Duration { return time.Millisecond }

func TestPipeline_HammerBudgetWithGlobalRetriesDisabled(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, hammer := range []bool{false, true} {
			name := "regular"
			if hammer {
				name = "hammer"
			}
			if stream {
				name += "_stream"
			}
			t.Run(name, func(t *testing.T) {
				calls, prepares, switches := 0, 0, 0
				attempt := func() error {
					calls++
					return &httpclient.Error{StatusCode: http.StatusTooManyRequests}
				}
				outbound := &disabledRetryBudgetOutbound{
					hammer: hammer,
					mockOutboundWithRetryLimit: mockOutboundWithRetryLimit{
						sameChannelLimit: func() (int, bool) { return 7, true },
						mockOutbound: mockOutbound{
							canRetry:        func(error) bool { return calls < 4 },
							prepareForRetry: func(context.Context) error { prepares++; return nil },
							hasMoreChannels: func() bool { return true },
							nextChannel:     func(context.Context) error { switches++; return nil },
						},
					},
				}
				p := &pipeline{
					Inbound: &mockInbound{transformRequest: func(context.Context, *httpclient.Request) (*llm.Request, error) {
						return &llm.Request{Stream: lo.ToPtr(stream)}, nil
					}},
					Outbound: outbound,
					Executor: &mockExecutor{
						do: func(context.Context, *httpclient.Request) (*httpclient.Response, error) { return nil, attempt() },
						doStream: func(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
							return nil, attempt()
						},
					},
				}
				WithRetry(2, 2, 0)(p)
				WithRetriesDisabled()(p)
				require.Equal(t, hammer, p.hasStreamRetryBudget())
				require.Zero(t, p.maxSameChannelRetriesFor(), "ordinary count overrides remain disabled")
				started := time.Now()
				_, err := p.Process(context.Background(), &httpclient.Request{})
				require.Error(t, err)
				require.Zero(t, switches)
				if hammer {
					require.Equal(t, 4, calls)
					require.Equal(t, 3, prepares)
					require.GreaterOrEqual(t, time.Since(started), 3*time.Millisecond)
				} else {
					require.Equal(t, 1, calls)
					require.Zero(t, prepares)
				}
			})
		}
	}
}

func TestPipeline_HammerPreCommitStreamRetryWithGlobalRetriesDisabled(t *testing.T) {
	attempts := 0
	outbound := &disabledRetryBudgetOutbound{hammer: true}
	outbound.canRetry = func(error) bool { return attempts < 4 }
	outbound.transformStream = func(context.Context, *httpclient.Request, streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
		// Metadata before a failure is not a committed response. Hammer retry must
		// keep probing it even while ordinary global retries are disabled.
		return &llmErrorAfterStream{items: []*llm.Response{llmEmptyChunk(), llmEmptyChunk(), llmEmptyChunk()}, err: &httpclient.Error{StatusCode: 429}}, nil
	}
	p := NewFactory(&mockExecutor{doStream: func(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
		attempts++
		return streams.SliceStream([]*httpclient.StreamEvent{}), nil
	}}).Pipeline(&mockInbound{transformRequest: func(context.Context, *httpclient.Request) (*llm.Request, error) {
		return &llm.Request{Stream: lo.ToPtr(true)}, nil
	}}, outbound, WithRetriesDisabled())
	_, err := p.Process(context.Background(), &httpclient.Request{})
	require.Error(t, err)
	require.Equal(t, 4, attempts)
}
