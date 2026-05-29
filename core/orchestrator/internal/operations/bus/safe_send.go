package bus

func safeSend(ch chan []byte, payload []byte) (sent bool) {
	defer func() {
		if recover() != nil {
			sent = false
		}
	}()
	select {
	case ch <- payload:
		return true
	default:
		return false
	}
}
