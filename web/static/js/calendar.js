document.addEventListener('DOMContentLoaded', function() {
    fetch('/tasks')
        .then(response => response.json())
        .then(data => {
            const calendarEl = document.getElementById('calendar');
            const calendar = new FullCalendar.Calendar(calendarEl, {
                initialView: 'dayGridMonth',
                headerToolbar: {
                    left: 'prev,next today',
                    center: 'title',
                    right: 'dayGridMonth,timeGridWeek,timeGridDay'
                },
                events: generateEvents(data)
            });

            calendar.render();

            document.getElementById('today-button').addEventListener('click', function() {
                calendar.today();
            });

            function generateEvents(tasks) {
                const events = [];
                tasks.forEach(task => {
                    if (task.is_recurring) {
                        const daysOfWeek = task.days_of_week.split(',').map(day => day.trim().toLowerCase());
                        const dayMap = {
                            "sun": 0,
                            "mon": 1,
                            "tue": 2,
                            "wed": 3,
                            "thu": 4,
                            "fri": 5,
                            "sat": 6
                        };
                        daysOfWeek.forEach(day => {
                            const dayIndex = dayMap[day];
                            if (dayIndex !== undefined) {
                                events.push({
                                    title: task.name,
                                    startRecur: new Date(),
                                    endRecur: new Date(new Date().getFullYear() + 1, 11, 31), // повторяется до конца года
                                    daysOfWeek: [dayIndex],
                                    startTime: task.time,
                                    description: task.description,
                                    extendedProps: {
                                        service: task.service,
                                        isRecurring: task.is_recurring
                                    }
                                });
                            }
                        });
                    } else {
                        events.push({
                            title: task.name,
                            start: getTaskStartDate(task),
                            allDay: false,
                            description: task.description,
                            extendedProps: {
                                service: task.service,
                                isRecurring: task.is_recurring
                            }
                        });
                    }
                });
                return events;
            }

            function getTaskStartDate(task) {
                const [hours, minutes] = task.time.split(':');
                const date = new Date();
                date.setHours(hours, minutes, 0, 0);
                return date;
            }
        });
});
