document.addEventListener('DOMContentLoaded', function() {
    fetch('/api/tasks')
        .then(response => response.json())
        .then(data => {
            const calendarEl = document.getElementById('calendar');
            const taskColors = generateTaskColors(data);

            const calendar = new FullCalendar.Calendar(calendarEl, {
                initialView: 'dayGridMonth',
                headerToolbar: {
                    left: 'prev,next today',
                    center: 'title',
                    right: 'dayGridMonth,timeGridWeek,timeGridDay'
                },
                height: 'auto',
                events: generateEvents(data),
                eventTimeFormat: { // like '14:30'
                    hour: '2-digit',
                    minute: '2-digit',
                    hour12: false
                },
                eventContent: function(info) {
                    return {
                        html: `<div style="background-color: ${taskColors[info.event.title]}; padding: 5px; border-radius: 5px; white-space: normal; overflow: hidden;">
                                <span>${info.timeText} - ${info.event.title}</span>
                              </div>`
                    };
                },
                slotMinTime: '00:00:00',
                slotMaxTime: '24:00:00',
                expandRows: true,
                dayMaxEventRows: true,
                dayMaxEvents: 3, // ограничение на количество отображаемых событий в ячейке
                views: {
                    dayGridMonth: {
                        dayMaxEventRows: 3 // ограничение на количество отображаемых событий в ячейке для месячного представления
                    }
                }
            });

            calendar.render();

            function generateTaskColors(tasks) {
                const colors = {};
                const colorPalette = [
                    '#FF5733', '#33FF57', '#3357FF', '#FF33A1', '#FF8C33', '#8C33FF', '#33FFBD', '#FF5733', '#33A1FF', '#FF33A1'
                ];
                let colorIndex = 0;

                tasks.forEach(task => {
                    if (!colors[task.name]) {
                        colors[task.name] = colorPalette[colorIndex % colorPalette.length];
                        colorIndex++;
                    }
                });

                return colors;
            }

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
                                    startTime: convertToUserTimeZone(task.time),
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

            function convertToUserTimeZone(time) {
                const userTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
                const date = new Date(`1970-01-01T${time}Z`);
                const options = { timeZone: userTimeZone, hour: '2-digit', minute: '2-digit', hour12: false };
                return date.toLocaleTimeString([], options);
            }
        });
});
