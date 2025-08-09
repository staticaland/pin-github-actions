# Variables
workflow := "sample.yml"

# Show proposed changes without applying (answers "n" to the prompt)
pin-dry:
	printf "n\n" | go run main.go {{workflow}}

# Apply changes (answers "y" to the prompt)
pin-apply:
	printf "y\n" | go run main.go {{workflow}}

# Convenience alias to run dry by default
run:
	just pin-dry
