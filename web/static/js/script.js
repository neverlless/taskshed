document.addEventListener('DOMContentLoaded', function() {
    fetch('/tasks')
        .then(response => response.json())
        .then(data => {
            const taskList = document.getElementById('task-list');
            const serviceFilter = document.getElementById('service-filter');
            const searchInput = document.getElementById('search');

            // Заполнить фильтр сервисов
            const services = [...new Set(data.map(task => task.service))];
            services.forEach(service => {
                const option = document.createElement('option');
                option.value = service;
                option.textContent = service;
                serviceFilter.appendChild(option);
            });

            // Функция для корректировки времени согласно временной зоне пользователя
            function convertToUserTimeZone(time) {
                const userTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
                const date = new Date(`1970-01-01T${time}Z`);
                const options = { timeZone: userTimeZone, hour: '2-digit', minute: '2-digit' };
                return date.toLocaleTimeString([], options);
            }

            // Функция для отображения задач
            function displayTasks(tasks) {
                taskList.innerHTML = '';
                tasks.forEach(task => {
                    const taskDiv = document.createElement('div');
                    taskDiv.className = 'task';
                    taskDiv.innerHTML = `
                        <h2>${task.name}</h2>
                        <p><strong>Service:</strong> ${task.service}</p>
                        <p><strong>Time:</strong> ${convertToUserTimeZone(task.time)}</p>
                        <p><strong>Days of Week:</strong> ${task.days_of_week}</p>
                        <p><strong>Is Recurring:</strong> ${task.is_recurring}</p>
                        <p><strong>Description:</strong> ${task.description}</p>
                    `;
                    taskList.appendChild(taskDiv);
                });
            }

            // Отображение всех задач при загрузке
            displayTasks(data);

            // Фильтрация задач
            function filterTasks() {
                const searchText = searchInput.value.toLowerCase();
                const selectedService = serviceFilter.value;
                const filteredTasks = data.filter(task => {
                    return (task.name.toLowerCase().includes(searchText) || task.service.toLowerCase().includes(searchText)) &&
                           (selectedService === '' || task.service === selectedService);
                });
                displayTasks(filteredTasks);
            }

            // Добавить обработчики событий для фильтров
            searchInput.addEventListener('input', filterTasks);
            serviceFilter.addEventListener('change', filterTasks);
        });
});
