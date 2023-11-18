package pages

func GetData() map[string][]byte {
	return map[string][]byte{
		"fooo": []byte("bar"),
		"Tom":  []byte("<h1>F*ck off!</h1>"),
	}
}
