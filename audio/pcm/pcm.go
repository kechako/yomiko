package pcm

import (
	"encoding/binary"
	"math"
)

type Type interface {
	int16 | float32
}

const Int16Scale = 32768

func int16ToFloat32(v int16) float32 {
	return float32(v) / Int16Scale
}

func float32ToInt16(v float32) int16 {
	v = v * Int16Scale
	v = max(v, math.MinInt16)
	v = min(v, math.MaxInt16)
	return int16(v)
}

func float32ArrayToInt16Array(dst []int16, src []float32) int {
	l := min(len(dst), len(src))
	for i := 0; i < l; i++ {
		dst[i] = float32ToInt16(src[i])
	}
	return l
}

func int16ArrayToFloat16Array(dst []float32, src []int16) int {
	l := min(len(dst), len(src))
	for i := 0; i < l; i++ {
		dst[i] = int16ToFloat32(src[i])
	}
	return l
}

type ByteOrder = binary.ByteOrder

var (
	LittleEndian ByteOrder = binary.LittleEndian
	BigEndian    ByteOrder = binary.BigEndian
)

func BytesToSamples[T Type](bytes int) int {
	var v T
	switch any(v).(type) {
	case int16:
		return bytes / 2
	case float32:
		return bytes / 4
	}

	// unreachable
	return 0
}

func SamplesToBytes[T Type](samples int) int {
	var v T
	switch any(v).(type) {
	case int16:
		return samples * 2
	case float32:
		return samples * 4
	}

	// unreachable
	return 0
}

func Bytes[T Type](data []T) int {
	return SamplesToBytes[T](len(data))
}

func Samples[T Type](b []byte) int {
	return BytesToSamples[T](len(b))
}

func Encode[T Type](p []byte, data []T, order ByteOrder) (int, error) {
	l := min(len(p), binary.Size(data))

	switch v := any(data).(type) {
	case []int16:
		n := l / 2
		for i, x := range v[:n] {
			order.PutUint16(p[2*i:], uint16(x))
		}
	case []float32:
		n := l / 4
		for i, x := range v[:n] {
			order.PutUint32(p[4*i:], math.Float32bits(x))
		}
	}

	return l, nil
}

func Decode[T Type](data []T, p []byte, order ByteOrder) (int, error) {
	l := min(len(p), binary.Size(data))

	switch v := any(data).(type) {
	case []int16:
		n := l / 2
		for i := range v[:n] {
			v[i] = int16(order.Uint16(p[2*i:]))
		}
	case []float32:
		n := l / 4
		for i := range v[:n] {
			v[i] = math.Float32frombits(order.Uint32(p[4*i:]))
		}
	}

	return l, nil
}
