package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// EncodePCM16ToWAV 将 PCM16 单声道数据封装为 WAV 格式。
func EncodePCM16ToWAV(samples []int16, sampleRate int) ([]byte, error) {
	if sampleRate <= 0 {
		return nil, fmt.Errorf("非法采样率: %d", sampleRate)
	}

	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	totalDataLen := uint32(len(samples)*2 + 36)
	if err := binary.Write(buf, binary.LittleEndian, totalDataLen); err != nil {
		return nil, err
	}
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	if err := binary.Write(buf, binary.LittleEndian, uint32(16)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(1)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(1)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return nil, err
	}
	byteRate := uint32(sampleRate * 2)
	if err := binary.Write(buf, binary.LittleEndian, byteRate); err != nil {
		return nil, err
	}
	blockAlign := uint16(2)
	if err := binary.Write(buf, binary.LittleEndian, blockAlign); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(16)); err != nil {
		return nil, err
	}

	buf.WriteString("data")
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(samples)*2)); err != nil {
		return nil, err
	}
	for _, sample := range samples {
		if err := binary.Write(buf, binary.LittleEndian, sample); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}
