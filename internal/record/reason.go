package record

func reason(field string) string {
	switch field {
	case "Op":
		return "неизвестная операция"
	case "Secret":
		return "пустой секрет"
	case "Note":
		return "пометка содержит символ U+0000"
	}
	return "сумма файла не на месте или неверного вида"
}
