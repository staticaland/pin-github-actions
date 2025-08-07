# Variables
workflow := "sample.yml"

# Show proposed changes without applying (answers "n" to the prompt)
pin-dry:
	printf "n\n" | go run main.go {{workflow}}

# Apply changes (answers "y" to the prompt)
pin-apply:
	printf "y\n" | go run main.go {{workflow}}

# Run tests
test:
	go test ./...

# Convenience alias to run dry by default
run:
	just pin-dry

# Run dry-run over all workflow files in the repo
pin-all-dry:
	for f in .github/workflows/*.yml .github/workflows/*.yaml; do
		[ -e "$$f" ] || continue
		echo "==> $$f"
		printf "n\n" | go run main.go "$$f"
	done
