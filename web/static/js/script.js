document.addEventListener('DOMContentLoaded', function() {
    fetch('/api/tasks')
        .then(response => response.json())
        .then(data => {
            const taskList = document.getElementById('task-list');
            const serviceFilter = document.getElementById('service-filter');
            const searchInput = document.getElementById('search');
            const viewToggle = document.getElementById('view-toggle');
            const downloadCsvButton = document.getElementById('download-csv');
            let isGridView = true;
            let currentSortColumn = '';
            let currentSortOrder = '';

            // Populate service filter
            const services = [...new Set(data.map(task => task.service))];
            services.forEach(service => {
                const option = document.createElement('option');
                option.value = service;
                option.textContent = service;
                serviceFilter.appendChild(option);
            });

            // Function to convert time to user's timezone
            function convertToUserTimeZone(time) {
                const userTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
                const date = new Date(`1970-01-01T${time}Z`);
                const options = { timeZone: userTimeZone, hour: '2-digit', minute: '2-digit', hour12: false };
                return date.toLocaleTimeString([], options);
            }

            // Function to display tasks in a grid
            function displayTasks(tasks) {
                taskList.innerHTML = '';
                tasks.forEach(task => {
                    const taskDiv = document.createElement('div');
                    taskDiv.className = 'task';
                    taskDiv.innerHTML = `
                        <h2>${task.name}</h2>
                        <p><strong>Service:</strong> ${task.service}</p>
                        <p><strong>Time:</strong> ${convertToUserTimeZone(task.time)}</p>
                        ${task.duration ? `<p><strong>Duration:</strong> ${task.duration}</p><p><strong>End Time:</strong> ${convertToUserTimeZone(getEndTime(task.time, task.duration))}</p>` : ''}
                        <p><strong>Days of Week:</strong> ${task.days_of_week}</p>
                        <p><strong>Is Recurring:</strong> ${task.is_recurring}</p>
                        <p><strong>Description:</strong> ${task.description}</p>
                        ${task.hosts ? `<p><strong>Hosts:</strong> ${task.hosts}</p>` : ''}
                    `;
                    taskList.appendChild(taskDiv);
                });
            }

            // Function to get end time of a task
            function getEndTime(startTime, duration) {
                const [startHours, startMinutes] = startTime.split(':').map(Number);
                const [durationHours, durationMinutes] = duration.split(':').map(Number);
                const endDate = new Date();
                endDate.setHours(startHours + durationHours, startMinutes + durationMinutes, 0, 0);
                return endDate.toTimeString().slice(0, 5); // Returns 'HH:mm'
            }

            // Function to display tasks in a table
            function displayTasksList(tasks) {
                taskList.innerHTML = '';
                const container = document.createElement('div');
                container.className = 'task-table-container';
                const table = document.createElement('table');
                table.className = 'task-table';
                table.innerHTML = `
                    <thead>
                        <tr>
                            <th data-column="name">Name</th>
                            <th data-column="service">Service</th>
                            <th data-column="time">Time</th>
                            <th data-column="duration">Duration</th>
                            <th data-column="end_time">End Time</th>
                            <th data-column="days_of_week">Days of Week</th>
                            <th data-column="is_recurring">Is Recurring</th>
                            <th data-column="description">Description</th>
                            <th data-column="hosts">Hosts</th>
                        </tr>
                    </thead>
                    <tbody>
                    </tbody>
                `;
                tasks.forEach(task => {
                    const row = document.createElement('tr');
                    row.innerHTML = `
                        <td>${task.name}</td>
                        <td>${task.service}</td>
                        <td>${convertToUserTimeZone(task.time)}</td>
                        <td>${task.duration || ''}</td>
                        <td>${task.duration ? convertToUserTimeZone(getEndTime(task.time, task.duration)) : ''}</td>
                        <td>${task.days_of_week}</td>
                        <td>${task.is_recurring}</td>
                        <td>${task.description}</td>
                        <td>${task.hosts || ''}</td>
                    `;
                    table.querySelector('tbody').appendChild(row);
                });
                container.appendChild(table);
                taskList.appendChild(container);

                // Add sorting functionality
                const headers = table.querySelectorAll('th');
                headers.forEach(header => {
                    header.addEventListener('click', () => {
                        const column = header.getAttribute('data-column');
                        if (currentSortColumn === column) {
                            currentSortOrder = currentSortOrder === 'asc' ? 'desc' : 'asc';
                        } else {
                            currentSortColumn = column;
                            currentSortOrder = 'asc';
                        }
                        headers.forEach(h => h.classList.remove('sorted-asc', 'sorted-desc'));
                        header.classList.add(currentSortOrder === 'asc' ? 'sorted-asc' : 'sorted-desc');
                        sortTasks(tasks, column, currentSortOrder);
                        displayTasksList(tasks);
                    });
                });
            }

            // Function to sort tasks
            function sortTasks(tasks, column, order) {
                tasks.sort((a, b) => {
                    if (a[column] < b[column]) return order === 'asc' ? -1 : 1;
                    if (a[column] > b[column]) return order === 'asc' ? 1 : -1;
                    return 0;
                });
            }

            // Function for downloading tasks as CSV
            function downloadCSV(tasks) {
                const csvContent = [
                    ['Name', 'Service', 'Time', 'Duration', 'End Time', 'Days of Week', 'Is Recurring', 'Description', 'Hosts'],
                    ...tasks.map(task => [
                        task.name,
                        task.service,
                        convertToUserTimeZone(task.time),
                        task.duration || '',
                        task.duration ? convertToUserTimeZone(getEndTime(task.time, task.duration)) : '',
                        task.days_of_week,
                        task.is_recurring,
                        task.description,
                        task.hosts || ''
                    ])
                ].map(e => e.join(";")).join("\n");

                const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
                const url = URL.createObjectURL(blob);
                const link = document.createElement('a');
                link.setAttribute('href', url);
                link.setAttribute('download', 'tasks.csv');
                link.style.visibility = 'hidden';
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
            }

            // Display tasks
            displayTasks(data);

            // Filter tasks
            function filterTasks() {
                const searchText = searchInput.value.toLowerCase();
                const selectedService = serviceFilter.value;
                const filteredTasks = data.filter(task => {
                    return (task.name.toLowerCase().includes(searchText) || task.service.toLowerCase().includes(searchText)) &&
                        (selectedService === '' || task.service === selectedService);
                });
                if (isGridView) {
                    displayTasks(filteredTasks);
                } else {
                    displayTasksList(filteredTasks);
                }
            }

            // Add event listeners
            searchInput.addEventListener('input', filterTasks);
            serviceFilter.addEventListener('change', filterTasks);

            // View toggle button
            viewToggle.addEventListener('click', function() {
                isGridView = !isGridView;
                filterTasks();
            });

            // Button to download tasks as CSV
            downloadCsvButton.addEventListener('click', function() {
                downloadCSV(data);
            });
        });
});
