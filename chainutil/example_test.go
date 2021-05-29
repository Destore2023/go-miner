package chainutil_test

import (
	"fmt"
	"math"

	"github.com/Sukhavati-Labs/go-miner/chainutil"
)

func ExampleAmount() {

	a := chainutil.ZeroAmount()
	fmt.Println("Zero Sukhavati:", a)

	a, _ = chainutil.NewAmountFromUint(100000000)
	fmt.Println("100,000,000 Sukhavati:", a)

	a, _ = chainutil.NewAmountFromUint(100000)
	fmt.Println("100,000 Sukhavati:", a)
	// Output:
	// Zero Sukhavati: 0 Skt
	// 100,000,000 Sukhavati: 1 Skt
	// 100,000 Sukhavati: 0.001 Skt
}

func ExampleNewAmountFromSkt() {
	amountOne, err := chainutil.NewAmountFromSkt(1)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(amountOne) //Output 1

	amountFraction, err := chainutil.NewAmountFromSkt(0.01234567)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(amountFraction) //Output 2

	amountZero, err := chainutil.NewAmountFromSkt(0)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(amountZero) //Output 3

	amountNaN, err := chainutil.NewAmountFromSkt(math.NaN())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(amountNaN) //Output 4

	// Output: 1 Skt
	// 0.01234567 Skt
	// 0 Skt
	// invalid float
}

func ExampleAmount_unitConversions() {
	amount, _ := chainutil.NewAmountFromUint(44433322211100)

	fmt.Println("Sukhavati to kSkt:", amount.Format(chainutil.AmountKiloSkt))
	fmt.Println("Sukhavati to Skt:", amount)
	fmt.Println("Sukhavati to MilliSkt:", amount.Format(chainutil.AmountMilliSkt))
	fmt.Println("Sukhavati to MicroSkt:", amount.Format(chainutil.AmountMicroSkt))
	fmt.Println("Sukhavati to Sukhavati:", amount.Format(chainutil.AmountSukhavati))

	// Output:
	// Sukhavati to kSkt: 444.333222111 kSkt
	// Sukhavati to Skt: 444333.222111 Skt
	// Sukhavati to MilliSkt: 444333222.111 mSkt
	// Sukhavati to MicroSkt: 444333222111 ¦ÌSkt
	// Sukhavati to Sukhavati: 44433322211100 Sukhavati
}
