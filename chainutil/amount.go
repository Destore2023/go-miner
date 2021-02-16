package chainutil

import (
	"errors"
	"math"
	"strconv"

	"github.com/Sukhavati-Labs/go-miner/chainutil/safetype"
	"github.com/Sukhavati-Labs/go-miner/consensus"
)

var (
	ErrInvalidFloat = errors.New("invalid float")
	ErrMaxAmount    = errors.New("exceed maximum amount")
)

// AmountUnit describes a method of converting an Amount to something
// other than the base unit of a Skt.  The value of the AmountUnit
// is the exponent component of the decadic multiple to convert from
// an amount in Skt to an amount counted in units.
type AmountUnit int

// These constants define various units used when describing a Skt
// monetary amount.
const (
	AmountMegaSkt   AmountUnit = 6
	AmountKiloSkt   AmountUnit = 3
	AmountSkt       AmountUnit = 0
	AmountMilliSkt  AmountUnit = -3
	AmountMicroSkt  AmountUnit = -6
	AmountSukhavati AmountUnit = -8
)

// String returns the unit as a string.  For recognized units, the SI
// prefix is used, or "Sukhavati" for the base unit.  For all unrecognized
// units, "1eN SKT" is returned, where N is the AmountUnit.
func (u AmountUnit) String() string {
	switch u {
	case AmountMegaSkt:
		return "MSKT"
	case AmountKiloSkt:
		return "kSKT"
	case AmountSkt:
		return "SKT"
	case AmountMilliSkt:
		return "mSKT"
	case AmountMicroSkt:
		return "μSKT"
	case AmountSukhavati:
		return "Sukhavati"
	default:
		return "1e" + strconv.FormatInt(int64(u), 10) + " SKT"
	}
}

// Amount represents the base Skt monetary unit (colloquially referred
// to as a `Sukhavati').  A single Amount is equal to 1e-8 of a Skt.
// The value is limited in range [0, consensus.MaxSkt * consensus.SukhavatiPerSkt]
type Amount struct {
	value *safetype.Uint128
}

var (
	zeroAmount    = Amount{safetype.NewUint128()}
	maxAmount     Amount
	minRelayTxFee Amount
)

func init() {
	u, err := safetype.NewUint128FromUint(consensus.MaxSkt).
		Mul(safetype.NewUint128FromUint(consensus.SukhavatiPerSkt))
	if err != nil {
		panic("init maxAmount error: " + err.Error())
	}

	maxAmount = Amount{u}

	minRelayTxFee, err = NewAmountFromUint(consensus.MinRelayTxFee)
	if err != nil {
		panic("init minRelayTxFee error: " + err.Error())
	}
}

// MaxAmount is the maximum transaction amount allowed in Sukhavati.
func MaxAmount() Amount {
	return maxAmount
}

func ZeroAmount() Amount {
	return zeroAmount
}

// MinRelayTxFee is the minimum fee required for relaying tx.
func MinRelayTxFee() Amount {
	return minRelayTxFee
}

// round converts a floating point number, which may or may not be representable
// as an integer, to the Amount integer type by rounding to the nearest integer.
// This is performed by adding or subtracting 0.5 depending on the sign, and
// relying on integer truncation to round the value to the nearest Amount.
func round(Sukhavati float64) (Amount, error) {
	var v int64
	if Sukhavati < 0 {
		v = int64(Sukhavati - 0.5)
	} else {
		v = int64(Sukhavati + 0.5)
	}
	u, err := safetype.NewUint128FromInt(v)
	if err != nil {
		return zeroAmount, err
	}
	return checkedAmount(u)
}

// NewAmountFromskt creates an Amount from a floating point value representing
// some value in skt.  NewAmountFromskt errors if f is NaN or +-Infinity, but
// does not check that the amount is within the total amount of skt
// producible as f may not refer to an amount at a single moment in time.
//
// NewAmountFromskt is for specifically for converting skt to Sukhavati.
// For creating a new Amount with an int64 value which denotes a quantity of Sukhavati,
// do a simple type conversion from type int64 to Amount.
func NewAmountFromSkt(f float64) (Amount, error) {
	// The amount is only considered invalid if it cannot be represented
	// as an integer type.  This may happen if f is NaN or +-Infinity.
	switch {
	case math.IsNaN(f):
		fallthrough
	case math.IsInf(f, 1):
		fallthrough
	case math.IsInf(f, -1):
		return zeroAmount, ErrInvalidFloat
	}
	return round(f * float64(consensus.SukhavatiPerSkt))
}

func NewAmountFromInt(i int64) (Amount, error) {
	u, err := safetype.NewUint128FromInt(i)
	if err != nil {
		return zeroAmount, err
	}
	return checkedAmount(u)
}

func NewAmountFromUint(i uint64) (Amount, error) {
	return NewAmount(safetype.NewUint128FromUint(i))
}

func NewAmount(u *safetype.Uint128) (Amount, error) {
	return checkedAmount(u)
}

// ToUnit converts a monetary amount counted in skt base units to a
// floating point value representing an amount of skt.
func (a Amount) ToUnit(u AmountUnit) float64 {
	return float64(a.UintValue()) / math.Pow10(int(u+8))
}

// Toskt is the equivalent of calling ToUnit with Amountskt.
func (a Amount) ToSkt() float64 {
	return a.ToUnit(AmountSkt)
}

// Format formats a monetary amount counted in skt base units as a
// string for a given unit.  The conversion will succeed for any unit,
// however, known units will be formated with an appended label describing
// the units with SI notation, or "Sukhavati" for the base unit.
func (a Amount) Format(u AmountUnit) string {
	units := " " + u.String()
	return strconv.FormatFloat(a.ToUnit(u), 'f', -int(u+8), 64) + units
}

// String is the equivalent of calling Format with Amountskt.
func (a Amount) String() string {
	return a.Format(AmountSkt)
}

// MulF64 multiplies an Amount by a floating point value.  While this is not
// an operation that must typically be done by a full node or wallet, it is
// useful for services that build on top of skt (for example, calculating
// a fee by multiplying by a percentage).
func (a Amount) MulF64(f float64) (Amount, error) {
	switch {
	case math.IsNaN(f):
		fallthrough
	case math.IsInf(f, 1):
		fallthrough
	case math.IsInf(f, -1):
		return zeroAmount, ErrInvalidFloat
	}

	return round(float64(a.UintValue()) * f)
}

func (a Amount) Add(b Amount) (Amount, error) {
	u, err := a.value.Add(b.value)
	if err != nil {
		return zeroAmount, err
	}
	return checkedAmount(u)
}

func (a Amount) AddInt(i int64) (Amount, error) {
	u, err := a.value.AddInt(i)
	if err != nil {
		return zeroAmount, err
	}
	return checkedAmount(u)
}

func (a Amount) Sub(b Amount) (Amount, error) {
	u, err := a.value.Sub(b.value)
	if err != nil {
		return zeroAmount, err
	}
	return checkedAmount(u)
}

func (a Amount) Cmp(b Amount) int {
	return a.value.Cmp(b.value)
}

func (a Amount) IsZero() bool {
	return a.value.IsZero()
}

func (a Amount) Value() *safetype.Uint128 {
	return a.value
}

func (a Amount) IntValue() int64 {
	return a.value.IntValue()
}

func (a Amount) UintValue() uint64 {
	return a.value.UintValue()
}

func checkedAmount(u *safetype.Uint128) (Amount, error) {
	if u.Gt(maxAmount.value) {
		return zeroAmount, ErrMaxAmount
	}
	return Amount{u}, nil
}

// TstSetMinRelayTxFee For testing purposes
func TstSetMinRelayTxFee(fee uint64) {
	var err error
	minRelayTxFee, err = NewAmountFromUint(fee)
	if err != nil {
		panic("TstSetMinRelayTxFee error: " + err.Error())
	}
}

// TstSetMaxAmount For testing purpose
func TstSetMaxAmount(skt float64) {
	if skt < 0 {
		panic("TstSetMaxAmount error: negative")
	}
	u, err := safetype.NewUint128FromInt(int64(skt + 0.5))
	if err != nil {
		panic("TstSetMaxAmount error: " + err.Error())
	}

	u, err = u.Mul(safetype.NewUint128FromUint(consensus.SukhavatiPerSkt))
	if err != nil {
		panic("TstSetMaxAmount error: " + err.Error())
	}
	maxAmount = Amount{u}
}
