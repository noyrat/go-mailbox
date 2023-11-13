package utils

import (
	"math/big"
	"math/rand"
)

func IndexOf[T comparable](arr []T, item T) int {
	for i, o := range arr {
		if o == item {
			return i
		}
	}
	return -1
}
func Filter[T any](arr []T, f func(T) bool) (ret []T) {
	ret = make([]T, 0)
	for _, o := range arr {
		if f(o) {
			ret = append(ret, o)
		}
	}
	return ret
}
func Reverse[T any](arr []T) (ret []T) {
	ret = make([]T, 0)
	l := len(arr)
	for i := range arr {
		ret = append(ret, arr[l-1-i])
	}
	return ret
}
func MapReduce[T any, Y any](arr []T, f func(T) Y) (ret []Y) {
	for _, o := range arr {
		ret = append(ret, f(o))
	}
	return ret
}
func Uint64ToBytes(number uint64) []byte {
	big := new(big.Int)
	big.SetUint64(number)
	return big.Bytes()
}
func RandStr(n int) string {
	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890-"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}
