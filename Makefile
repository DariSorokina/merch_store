export TEST_DATABASE_PORT=5433
export TEST_DATABASE_USER=postgres
export TEST_DATABASE_PASSWORD=password
export TEST_DATABASE_NAME=shop
export TEST_DATABASE_HOST=db
export TEST_DATABASE_URI=host=127.0.0.1 port=5433 user=postgres password=password dbname=shop sslmode=disable


test.integration:
	docker run --rm -d --name $$TEST_DATABASE_HOST -e POSTGRES_USER=$$TEST_DATABASE_USER -e POSTGRES_PASSWORD=$$TEST_DATABASE_PASSWORD -e POSTGRES_DB=$$TEST_DATABASE_NAME -p $$TEST_DATABASE_PORT:5432 -v ./internal/storage/migrations/init.sql:/docker-entrypoint-initdb.d/init.sql postgres:13
	go test -v ./tests/integration/
	docker stop $$TEST_DATABASE_HOST