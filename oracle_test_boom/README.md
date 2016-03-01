```bash
$ oracle -pos=main.go:#40 callers github.com/tschottdorf/goplay/oracle_test_boom/
/Users/tschottdorf/go/src/github.com/tschottdorf/goplay/oracle_test_boom/main_test.go:10:2: invalid operation: (&(main.X literal)) (value of type *github.com/tschottdorf/goplay/oracle_test_boom.X) has no field or method Foo
oracle: couldn't load packages due to errors: github.com/tschottdorf/goplay/oracle_test_boom/_test
```
