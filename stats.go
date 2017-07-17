package modbus

type RequestStats struct {
	Num struct {
		All       int
		Invalid   int
		Timeout   int
		Exception int
		Other     int
	}
}

func (st *RequestStats) Percentage(num int) float64 {
	return 100 * float64(num) / float64(st.Num.All)
}

func (st *RequestStats) Update(err error) {
	st.Num.All++
	if err != nil {
		if _, ok := err.(Exception); ok {
			st.Num.Exception++
		} else if MsgInvalid(err) {
			st.Num.Invalid++
		} else if err == ErrTimeout {
			st.Num.Timeout++
		} else {
			st.Num.Other++
		}
	}
}
