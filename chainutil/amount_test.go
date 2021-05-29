package chainutil_test

import (
	"math"
	"strconv"
	"testing"

	. "github.com/Sukhavati-Labs/go-miner/chainutil"
	"github.com/Sukhavati-Labs/go-miner/chainutil/safetype"
	"github.com/Sukhavati-Labs/go-miner/consensus"
	"github.com/stretchr/testify/assert"
)

func TestAmountCreation(t *testing.T) {
	tests := []struct {
		name     string
		skt      float64
		valid    bool
		expected uint64
	}{
		// Positive tests.
		{
			name:     "zero",
			skt:      0,
			valid:    true,
			expected: 0,
		},
		{
			name:     "max producible",
			skt:      float64(consensus.MaxSkt),
			valid:    true,
			expected: MaxAmount().UintValue(),
		},
		{
			name:  "exceeds max producible",
			skt:   float64(consensus.MaxSkt) + 1e-7,
			valid: false,
		},
		{
			name:     "one hundred",
			skt:      100,
			valid:    true,
			expected: (100 * consensus.SukhavatiPerSkt),
		},
		{
			name:     "fraction",
			skt:      0.01234567,
			valid:    true,
			expected: 1234567,
		},
		{
			name:     "rounding up 1",
			skt:      54.999999999999943157,
			valid:    true,
			expected: 5500000000,
		},
		{
			name:     "rounding up 2",
			skt:      54.999999995000001357,
			valid:    true,
			expected: 5500000000,
		},
		{
			name:     "rounding down 1",
			skt:      54.999999994987654321,
			valid:    true,
			expected: 5499999999,
		},
		{
			name:     "rounding down 2",
			skt:      55.000000000000056843,
			valid:    true,
			expected: 5500000000,
		},

		// Negative tests.
		{
			name:  "not-a-number",
			skt:   math.NaN(),
			valid: false,
		},
		{
			name:  "-infinity",
			skt:   math.Inf(-1),
			valid: false,
		},
		{
			name:  "+infinity",
			skt:   math.Inf(1),
			valid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a, err := NewAmountFromSkt(test.skt)
			if test.valid {
				assert.Nil(t, err)
				if a.UintValue() != test.expected {
					t.Fatalf("expect %d but got %d", test.expected, a.UintValue())
				}
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}

func TestAmountUnitConversions(t *testing.T) {

	amt, err := NewAmountFromUint(44433322211100)
	assert.Nil(t, err)

	tests := []struct {
		name      string
		amount    Amount
		unit      AmountUnit
		converted float64
		s         string
	}{
		{
			name:      "MSkt",
			amount:    MaxAmount(),
			unit:      AmountMegaSkt,
			converted: float64(consensus.MaxSkt) / 1000000,
			s:         strconv.FormatFloat(float64(consensus.MaxSkt)/1000000, 'f', -1, 64) + " MSkt",
		},
		{
			name:      "kSkt",
			amount:    amt,
			unit:      AmountKiloSkt,
			converted: 444.33322211100,
			s:         "444.333222111 kSkt",
		},
		{
			name:      "Skt",
			amount:    amt,
			unit:      AmountSkt,
			converted: 444333.22211100,
			s:         "444333.222111 Skt",
		},
		{
			name:      "mSkt",
			amount:    amt,
			unit:      AmountMilliSkt,
			converted: 444333222.11100,
			s:         "444333222.111 mSkt",
		},
		{

			name:      "μSkt",
			amount:    amt,
			unit:      AmountMicroSkt,
			converted: 444333222111.00,
			s:         "444333222111 μSkt",
		},
		{

			name:      "sukhavati",
			amount:    amt,
			unit:      AmountSukhavati,
			converted: 44433322211100,
			s:         "44433322211100 Sukhavati",
		},
		{

			name:      "non-standard unit",
			amount:    amt,
			unit:      AmountUnit(-1),
			converted: 4443332.2211100,
			s:         "4443332.22111 1e-1 Skt",
		},
	}

	for _, test := range tests {
		f := test.amount.ToUnit(test.unit)
		if f != test.converted {
			t.Errorf("%v: converted value %v does not match expected %v", test.name, f, test.converted)
			continue
		}

		s := test.amount.Format(test.unit)
		if s != test.s {
			t.Errorf("%v: format '%v' does not match expected '%v'", test.name, s, test.s)
			continue
		}

		// Verify that Amount.ToSkt works as advertised.
		f1 := test.amount.ToUnit(AmountSkt)
		f2 := test.amount.ToSkt()
		if f1 != f2 {
			t.Errorf("%v: ToSkt does not match ToUnit(AmountSkt): %v != %v", test.name, f1, f2)
		}

		// Verify that Amount.String works as advertised.
		s1 := test.amount.Format(AmountSkt)
		s2 := test.amount.String()
		if s1 != s2 {
			t.Errorf("%v: String does not match Format(AmountSkt): %v != %v", test.name, s1, s2)
		}
	}
}

func TestAmountMulF64(t *testing.T) {
	tests := []struct {
		name string
		amt  uint64
		mul  float64
		res  uint64
		err  error
	}{
		{
			name: "Multiply 0.1 Skt by 2",
			amt:  10000000, // 0.1 Skt
			mul:  2,
			res:  20000000, // 0.2 Skt
		},
		{
			name: "Multiply 0.2 Skt by 0.02",
			amt:  20000000, // 0.2 Skt
			mul:  1.02,
			res:  20400000, // 0.204 Skt
		},
		{
			name: "Multiply 0.1 Skt by -2",
			amt:  10000000, // 0.1 Skt
			mul:  -1.02,
			err:  safetype.ErrUint128Underflow,
		},
		{
			name: "Multiply overflow",
			amt:  100000001, // 1.00000001 Skt
			mul:  float64(consensus.MaxSkt),
			err:  ErrMaxAmount,
		},
		{
			name: "Round down",
			amt:  49, // 49 Sukhavatis
			mul:  0.01,
			res:  0,
		},
		{
			name: "Round up",
			amt:  50, // 50 Sukhavatis
			mul:  0.01,
			res:  1, // 1 Sukhavati
		},
		{
			name: "Multiply by 0.",
			amt:  100000000, // 1 Skt
			mul:  0,
			res:  0, // 0 Skt
		},
		{
			name: "Multiply 1 by 0.5.",
			amt:  1, // 1 Sukhavati
			mul:  0.5,
			res:  1, // 1 Sukhavati
		},
		{
			name: "Multiply 100 by 66%.",
			amt:  100, // 100 Sukhavatis
			mul:  0.66,
			res:  66, // 66 Sukhavatis
		},
		{
			name: "Multiply 100 by 66.6%.",
			amt:  100, // 100 Sukhavatis
			mul:  0.666,
			res:  67, // 67 Sukhavatis
		},
		{
			name: "Multiply 100 by 2/3.",
			amt:  100, // 100 Sukhavatis
			mul:  2.0 / 3,
			res:  67, // 67 Sukhavatis
		},
	}

	for _, test := range tests {
		a, err := NewAmountFromUint(test.amt)
		assert.Nil(t, err)
		r, err := a.MulF64(test.mul)
		assert.Equal(t, test.err, err)
		if err != nil {
			continue
		}

		b, err := NewAmountFromUint(test.res)
		assert.Nil(t, err)
		assert.Equal(t, r.UintValue(), b.UintValue())
	}
}

func TestAdd(t *testing.T) {
	a, err := NewAmountFromUint(1)
	assert.Nil(t, err)
	_, err = MaxAmount().Add(a)
	assert.Equal(t, ErrMaxAmount, err)
}

func TestSub(t *testing.T) {
	a, err := NewAmountFromUint(0)
	assert.Nil(t, err)
	b, err := NewAmountFromUint(1)
	assert.Nil(t, err)
	c, err := NewAmountFromUint(0)
	assert.Nil(t, err)

	_, err = a.Sub(b)
	assert.Equal(t, safetype.ErrUint128Underflow, err)

	v1, err := a.Sub(c)
	assert.Nil(t, err)
	assert.Equal(t, uint64(0), v1.UintValue())

	v2, err := b.Sub(c)
	assert.Nil(t, err)
	assert.Equal(t, uint64(1), v2.UintValue())
}
