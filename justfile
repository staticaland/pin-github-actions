# Variables
workflow := "sample.yml"

# Show proposed changes without applying (answers "n" to the prompt)
pin-dry:
	printf "n\n" | go run main.go {{workflow}}

# Apply changes (answers "y" to the prompt)
pin-apply:
	printf "y\n" | go run main.go {{workflow}}

# Apply changes non-interactively using --yes
pin-apply-yes:
	go run main.go --yes {{workflow}}

# Run tests
test:
	go test ./...

# Convenience alias to run dry by default
run:
	just pin-dry
