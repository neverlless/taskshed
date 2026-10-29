document.addEventListener('DOMContentLoaded', async function() {
    const calendarEl = document.getElementById('calendar');
    const legendEl = document.getElementById('legend');
    showTimezone();

    let tasks;
    try {
        tasks = await fetchTasks();
    } catch (error) {
        showMessage(calendarEl, 'The schedule could not be loaded.', `${error.message} Check that the TaskShed server is running, then reload the page.`,
            el('button', { className: 'button', type: 'button', onclick: () => location.reload() }, 'Reload'));
        return;
    }

    // Colour by service so related jobs read as a group; all hues are light enough for dark event text.
    const palette = ['#8DB8F2', '#93D3A2', '#E9A6C9', '#C2AEF5', '#7FD4CF', '#F2AE8A', '#CFD879', '#B6C2D1'];
    const services = [...new Set(tasks.map(task => task.service))].sort();
    const colorOf = service => palette[services.indexOf(service) % palette.length];
    legendEl.replaceChildren(...services.map(service =>
        el('li', {}, el('span', { className: 'swatch', style: { background: colorOf(service) } }), service)));

    const events = tasks.flatMap(task => {
        const loc = localizeTask(task);
        // One-off tasks carry no date, so there is no day to place them on.
        if (!loc || !task.is_recurring || loc.days.length === 0) return [];
        return [{
            title: task.name,
            daysOfWeek: loc.days,
            startTime: formatSpan(loc.start),
            endTime: formatSpan(loc.end ?? loc.start + 60),
            backgroundColor: colorOf(task.service),
            borderColor: colorOf(task.service),
            textColor: '#141A22',
            extendedProps: { task, loc },
        }];
    });

    if (tasks.length === 0) {
        showMessage(calendarEl, 'No tasks scheduled yet.', 'Register a task with POST /api/tasks, as described in the README, and it will appear here.');
        return;
    }

    const narrow = window.matchMedia('(max-width: 640px)').matches;
    const time24 = { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' };
    new FullCalendar.Calendar(calendarEl, {
        initialView: narrow ? 'timeGridDay' : 'timeGridWeek',
        firstDay: 1,
        headerToolbar: {
            left: 'prev,next today',
            center: 'title',
            right: 'timeGridDay,timeGridWeek,dayGridMonth',
        },
        buttonText: { today: 'Today', day: 'Day', week: 'Week', month: 'Month' },
        height: 'auto',
        allDaySlot: false,
        nowIndicator: true,
        slotDuration: '01:00:00',
        slotLabelFormat: time24,
        eventTimeFormat: time24,
        displayEventEnd: true,
        dayMaxEvents: 3,
        events,
        eventDidMount(info) {
            const { task, loc } = info.event.extendedProps;
            const window = loc.end === null ? `${formatHM(loc.start)}, no set duration` : `${formatHM(loc.start)}–${formatHM(loc.end)}`;
            info.el.title = [task.name, task.service, window, task.hosts && `Hosts: ${task.hosts}`, task.description]
                .filter(Boolean).join('\n');
        },
    }).render();
});
