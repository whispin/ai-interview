package audio

// linearResample 执行简单的线性重采样。
func linearResample(samples []int16, fromRate, toRate int) []int16 {
	if fromRate == toRate || len(samples) == 0 {
		dup := make([]int16, len(samples))
		copy(dup, samples)
		return dup
	}

	ratio := float64(toRate) / float64(fromRate)
	targetLen := int(float64(len(samples)) * ratio)
	if targetLen <= 0 {
		targetLen = 1
	}

	res := make([]int16, targetLen)
	for i := range res {
		exact := float64(i) / ratio
		idx := int(exact)
		next := idx + 1
		if next >= len(samples) {
			next = len(samples) - 1
		}
		frac := exact - float64(idx)
		res[i] = int16(float64(samples[idx])*(1-frac) + float64(samples[next])*frac)
	}
	return res
}
