document.addEventListener('DOMContentLoaded', function() {
    fetch('/api/tasks')
        .then(response => response.json())
        .then(tasks => {
            const calendarEl = document.getElementById('calendar');
            const taskColors = generateTaskColors(tasks);

            const calendar = new FullCalendar.Calendar(calendarEl, {
                timeZone: 'local',  // Ensuring that the calendar uses the local timezone
                initialView: 'dayGridMonth',
                headerToolbar: {
                    left: 'prev,next today',
                    center: 'title',
                    right: 'dayGridMonth,timeGridWeek,timeGridDay'
                },
                height: 'auto',
                events: generateEvents(tasks),
                eventTimeFormat: {
                    hour: '2-digit',
                    minute: '2-digit',
                    hour12: false
                },
                displayEventTime: true,
                displayEventEnd: true,
                forceEventDuration: false,  // Ensure all events have a duration
                defaultTimedEventDuration: '01:00',  // Default duration for events without an end time
                eventDisplay: "block",
                eventContent: function(info) {
                    const endTimeText = info.event.end ? ` - ${info.event.end.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })}` : '';
                    return {
                        html: `<div style="background-color: ${taskColors[info.event.title]}; padding: 5px; border-radius: 5px; white-space: normal; overflow: hidden;">
                                <span>${info.timeText} ${info.event.title}</span>
                            </div>`
                    };
                },
                slotMinTime: '00:00:00',
                slotMaxTime: '24:00:00',
                expandRows: true,
                dayMaxEventRows: true,
                dayMaxEvents: 3,
                views: {
                    dayGridMonth: {
                        dayMaxEventRows: 3,
                        displayEventEnd: true
                    },
                    timeGridWeek: {
                        displayEventEnd: true
                    },
                    timeGridDay: {
                        displayEventEnd: true
                    }
                }
            });

            calendar.render();

            function generateTaskColors(tasks) {
                const colors = {};
                const colorPalette = [
                    '#FF5733', '#3357FF', '#FF33A1', '#FF8C33', '#8C33FF', '#FF5733', '#33A1FF', '#FF33A1', '#3f0d88', '#ab7af2'
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
                const defaultDuration = '01:00'; // Default duration of 1 hour

                tasks.forEach(task => {
                    if (!task.time || !task.name) {
                        console.error(`Skipping task due to missing required properties: ${JSON.stringify(task)}`);
                        return;
                    }

                    const startTime = convertToUserTimeZone(task.time);
                    const endTime = task.duration ? convertToUserTimeZone(getTaskEndTime(task.time, task.duration)) : convertToUserTimeZone(getTaskEndTime(task.time, defaultDuration));

                    const event = {
                        title: task.name,
                        start: startTime,
                        end: endTime,
                        allDay: false,
                        extendedProps: {
                            service: task.service,
                            isRecurring: task.is_recurring
                        }
                    };

                    if (task.is_recurring && task.days_of_week) {
                        const daysOfWeek = parseDaysOfWeek(task.days_of_week);
                        if (daysOfWeek.length > 0) {
                            event.daysOfWeek = daysOfWeek;
                            event.startTime = startTime; // 'HH:mm'
                            event.endTime = endTime; // 'HH:mm'
                            event.startRecur = new Date().toISOString().slice(0, 10); // 'YYYY-MM-DD'
                            event.endRecur = new Date(new Date().getFullYear() + 1, 11, 31).toISOString().slice(0, 10); // 'YYYY-MM-DD'
                        } else {
                            console.error(`Invalid days_of_week for recurring task: ${task.days_of_week}`);
                        }
                    }

                    console.log(`Generated event: ${JSON.stringify(event)}`);
                    events.push(event);
                });
                return events;
            }

            function getTaskStartTime(time) {
                return new Date().toISOString().slice(0, 11) + time + ":00Z"; // Combines current date with time and sets as UTC
            }

            function getTaskEndTime(startTime, duration) {
                const [hours, minutes] = startTime.split(':').map(Number);
                const [durHours, durMinutes] = duration.split(':').map(Number);
                const date = new Date();
                date.setUTCHours(hours + durHours, minutes + durMinutes, 0, 0);
                return date.toISOString().slice(11, 16); // returns 'HH:mm'
            }

            function parseDaysOfWeek(days) {
                const dayMap = {
                    "sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6
                };
                return days.split(',').map(day => dayMap[day.trim().toLowerCase()]).filter(day => day !== undefined);
            }

            function getTaskStartDate(time) {
                try {
                    const [hours, minutes] = time.split(':').map(Number);
                    if (isNaN(hours) || isNaN(minutes)) {
                        console.error(`Invalid start time format: ${time}`);
                        return null;
                    }
                    const date = new Date();
                    date.setUTCHours(hours, minutes, 0, 0);
                    return date;
                } catch (error) {
                    console.error(`Error in getTaskStartDate: ${error.message}`);
                    return null;
                }
            }

            function getTaskEndDate(startTime, duration) {
                try {
                    const [startHours, startMinutes] = startTime.split(':').map(Number);
                    const [durationHours, durationMinutes] = duration.split(':').map(Number);
                    if (isNaN(startHours) || isNaN(startMinutes) || isNaN(durationHours) || isNaN(durationMinutes)) {
                        console.error(`Invalid time or duration format: startTime=${startTime}, duration=${duration}`);
                        return null;
                    }
                    const startDate = new Date();
                    startDate.setUTCHours(startHours, startMinutes, 0, 0);
                    const endDate = new Date(startDate);
                    endDate.setUTCHours(startDate.getUTCHours() + durationHours, startDate.getUTCMinutes() + durationMinutes);
                    return endDate.toISOString();
                } catch (error) {
                    console.error(`Error in getTaskEndDate: ${error.message}`);
                    return null;
                }
            }

            function convertToUserTimeZone(time) {
                const userTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
                const date = new Date(`1970-01-01T${time}Z`);
                const options = { timeZone: userTimeZone, hour: '2-digit', minute: '2-digit', hour12: false };
                return date.toLocaleTimeString([], options);
            }
        });
});
