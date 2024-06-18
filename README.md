# TaskShed

TaskShed is a versatile task scheduling and management tool, designed to provide a centralized view of distributed system schedules. It offers a web interface for visualizing task schedules and an API for managing tasks.

## Key Features

- Centralized task scheduling view
- Web interface with calendar view
- Support for both SQLite and PostgreSQL databases
- RESTful API for task management
- Modern and intuitive UI

## Why Use TaskShed

TaskShed simplifies the management of distributed system schedules by providing a single point of access. It is easy to set up and use, with support for both SQLite and PostgreSQL. The web interface offers a clear and modern way to visualize tasks, making it easier to track and manage schedules.

## Installation

### From Source

1. Clone the repository:

    ```sh
    git clone https://github.com/neverlless/taskshed.git
    cd taskshed
    ```

2. Build the application:

    ```sh
    go build -o taskshed cmd/server/main.go
    ```

3. Run the application:

    ```sh
    ./taskshed --port=8080
    ```

### Using Binary

1. Download the latest release from the [releases page](https://github.com/neverlless/taskshed/releases).
2. Extract the archive:

    ```sh
    tar -xzf taskshed-vX.Y.Z-linux-amd64.tar.gz
    cd taskshed-vX.Y.Z-linux-amd64
    ```

3. Make the binary executable:

    ```sh
    chmod +x taskshed
    ```

4. Run the application:

    ```sh
    ./taskshed --port=8080
    ```

### Using Docker

1. Pull the Docker image:

    ```sh
    docker pull neverlless/taskshed
    ```

2. Run the Docker container:

    ```sh
    docker run -d -p 8080:8080 --name taskshed neverlless/taskshed
    ```

## Environment Variables

- `DB_TYPE`: Database type (`sqlite` or `postgres`)
- `DB_HOST`: Database host
- `DB_PORT`: Database port
- `DB_NAME`: Database name
- `DB_USER`: Database user
- `DB_PASSWORD`: Database password
  
## API Documentation

### Create Task

- Endpoint: POST `/tasks`
- Request Body:

    ```json
    {
        "name": "Backup Database",
        "service": "Database Service",
        "time": "03:00",
        "days_of_week": "Mon,Wed,Fri",
        "is_recurring": true,
        "description": "Daily backup of the database"
    }
    ```

- Response Body:

    ```json
    {
        "id": 1,
        "name": "Backup Database",
        "service": "Database Service",
        "time": "03:00",
        "days_of_week": "Mon,Wed,Fri",
        "is_recurring": true,
        "description": "Daily backup of the database"
    }
    ```

### Update Task

- Endpoint: PUT `/tasks/{id}`
- Request Body:

    ```json
    {
        "name": "Backup Database",
        "service": "Database Service",
        "time": "03:00",
        "days_of_week": "Mon,Wed,Fri",
        "is_recurring": true,
        "description": "Daily backup of the database"
    }
    ```

- Response Body:

    ```json
    {
        "id": 1,
        "name": "Backup Database",
        "service": "Database Service",
        "time": "03:00",
        "days_of_week": "Mon,Wed,Fri",
        "is_recurring": true,
        "description": "Daily backup of the database"
    }
    ```

### Delete Task

- Endpoint: DELETE `/tasks/{id}`
- Response Body: 204 No Content

### Get Task

- Endpoint: GET `/tasks`
- Response Body:

    ```json
    [
        {
            "id": 1,
            "name": "Backup Database",
            "service": "Database Service",
            "time": "03:00",
            "days_of_week": "Mon,Wed,Fri",
            "is_recurring": true,
            "description": "Daily backup of the database"
        }
    ]
    ```

## Screenshots

- Home page: ![Calendar View](screenshots/calendar.png)
- Calendar View: ![Calendar View](screenshots/calendar.png)

## Example cURL Commands

- Create Task:

    ```sh
    curl -X POST http://localhost:8080/tasks -H "Content-Type: application/json" -d '{
        "name": "Backup Database",
        "service": "Database Service",
        "time": "03:00",
        "days_of_week": "Mon,Wed,Fri",
        "is_recurring": true,
        "description": "Daily backup of the database"
    }'
    ```

- Update Task:

    ```sh
    curl -X PUT http://localhost:8080/tasks/1 -H "Content-Type: application/json" -d '{
        "name": "Backup Database",
        "service": "Database Service",
        "time": "03:00",
        "days_of_week": "Mon,Wed,Fri",
        "is_recurring": true,
        "description": "Daily backup of the database"
    }'
    ```

- Delete Task:

    ```sh
    curl -X DELETE http://localhost:8080/tasks/1
    ```

- Get Task:

    ```sh
    curl http://localhost:8080/tasks
    ```
