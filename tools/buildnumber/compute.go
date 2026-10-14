package main

func compute(s string) (string, error) {
	if s == "dev" {
		return s, nil
	}
	v, err := parse(s)
	if err != nil {
		return "", err
	}
	list, err := tags()
	if err != nil {
		return "", err
	}
	n, err := commits(base(list, v))
	if err != nil {
		return "", err
	}
	return format(v, n), nil
}
