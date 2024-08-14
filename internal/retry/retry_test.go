package retry

import (
	"context"
	"errors"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/require"

	retry_mocks "github.com/kiteggrad/tcontainer/internal/retry/mocks"
)

func Test_Retry(t *testing.T) {
	t.Parallel()

	type testMocks struct {
		operation      *retry_mocks.Operation
		backOffContext *retry_mocks.BackOffContext
	}
	type want struct {
		errors []error
	}
	type testCase struct {
		name      string
		wantCalls func(testMocks testMocks)
		want      want
	}
	type prepareTestCase func(t *testing.T) testCase
	testCases := []prepareTestCase{
		func(t *testing.T) testCase {
			// prepare case data
			prevErr := errors.New("previous error")

			testCase := testCase{
				name: "ok_with_second_try",
				wantCalls: func(testMocks testMocks) {
					testMocks.backOffContext.EXPECT().Context().
						Return(context.Background()).
						Maybe()
					testMocks.backOffContext.EXPECT().Reset().
						Maybe()

					testMocks.backOffContext.EXPECT().NextBackOff().
						Return(1).
						Once()

					testMocks.operation.EXPECT().Execute().Return(prevErr).Once()
					testMocks.operation.EXPECT().Execute().Return(nil).Once()
				},
				want: want{
					errors: nil,
				},
			}

			return testCase
		},
		func(t *testing.T) testCase {
			// prepare case data
			prevErr := errors.New("previous error")
			finalErr := errors.New("final error")

			testCase := testCase{
				name: "previous_and_final_error",
				wantCalls: func(testMocks testMocks) {
					testMocks.backOffContext.EXPECT().Context().
						Return(context.Background()).
						Maybe()
					testMocks.backOffContext.EXPECT().Reset().
						Maybe()

					testMocks.backOffContext.EXPECT().NextBackOff().
						Return(1).
						Twice()
					testMocks.backOffContext.EXPECT().NextBackOff().
						Return(backoff.Stop).
						Once()

					testMocks.operation.EXPECT().Execute().Return(prevErr).Twice()
					testMocks.operation.EXPECT().Execute().Return(finalErr).Once()
				},
				want: want{
					errors: []error{prevErr, finalErr},
				},
			}

			return testCase
		},
		func(t *testing.T) testCase {
			// prepare case data
			finalErr := errors.New("final error")

			testCase := testCase{
				name: "only_final_error",
				wantCalls: func(testMocks testMocks) {
					testMocks.backOffContext.EXPECT().Context().
						Return(context.Background()).
						Maybe()
					testMocks.backOffContext.EXPECT().Reset().
						Maybe()

					testMocks.backOffContext.EXPECT().NextBackOff().
						Return(backoff.Stop).
						Once()

					testMocks.operation.EXPECT().Execute().Return(finalErr).Once()
				},
				want: want{
					errors: []error{finalErr},
				},
			}

			return testCase
		},
	}
	for _, prepareTestCase := range testCases {
		test := prepareTestCase(t)
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require := require.New(t)

			// init entities
			testMocks := testMocks{
				operation:      retry_mocks.NewOperation(t),
				backOffContext: retry_mocks.NewBackOffContext(t),
			}
			test.wantCalls(testMocks)

			// run logic
			err := Retry(testMocks.operation.Execute, testMocks.backOffContext)
			if len(test.want.errors) == 0 {
				require.NoError(err)
			} else {
				for _, wantErr := range test.want.errors {
					require.ErrorIs(err, wantErr, err)
				}
			}
		})
	}
}
