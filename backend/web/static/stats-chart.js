(function () {
    const canvas = document.getElementById("stats-chart");
    const dataElement = document.getElementById("stats-chart-data");
    if (!canvas || !dataElement || typeof Chart === "undefined") return;

    const data = JSON.parse(dataElement.textContent);
    if (!data || data.length === 0) return;

    const series = (field) =>
        data.map((point) => ({
            x: Date.parse(point.time),
            y: point[field],
        }));
    const formatDate = (value) =>
        new Intl.DateTimeFormat(undefined, {
            year: "numeric",
            month: "short",
            day: "numeric",
            timeZone: "UTC",
        }).format(new Date(Number(value)));
    const formatDateTime = (value) =>
        new Intl.DateTimeFormat(undefined, {
            dateStyle: "medium",
            timeStyle: "short",
            timeZone: "UTC",
        }).format(new Date(Number(value)));
    const formatCompact = (value) =>
        new Intl.NumberFormat(undefined, {
            notation: "compact",
            maximumFractionDigits: 1,
        }).format(value);
    const formatNumber = (value) => new Intl.NumberFormat().format(value);

    const chart = new Chart(canvas, {
        type: "line",
        data: {
            datasets: [
                {
                    field: "pages",
                    label: "Pages",
                    data: series("pages"),
                    borderColor: "#3b82f6",
                    backgroundColor: "rgba(59,130,246,0.1)",
                    fill: false,
                    tension: 0.1,
                    yAxisID: "counts",
                },
                {
                    field: "articles",
                    label: "Articles",
                    data: series("articles"),
                    borderColor: "#8b5cf6",
                    backgroundColor: "rgba(139,92,246,0.1)",
                    fill: false,
                    tension: 0.1,
                    yAxisID: "counts",
                },
                {
                    field: "edits",
                    label: "Edits",
                    data: series("edits"),
                    borderColor: "#f59e0b",
                    backgroundColor: "rgba(245,158,11,0.1)",
                    fill: false,
                    tension: 0.1,
                    yAxisID: "counts",
                },
                {
                    field: "images",
                    label: "Images",
                    data: series("images"),
                    borderColor: "#10b981",
                    backgroundColor: "rgba(16,185,129,0.1)",
                    fill: false,
                    tension: 0.1,
                    yAxisID: "counts",
                },
                {
                    field: "users",
                    label: "Users",
                    data: series("users"),
                    borderColor: "#f43f5e",
                    backgroundColor: "rgba(244,63,94,0.1)",
                    fill: false,
                    tension: 0.1,
                    yAxisID: "users",
                },
            ],
        },
        options: {
            responsive: true,
            maintainAspectRatio: !canvas.hasAttribute("data-fill-container"),
            parsing: false,
            normalized: true,
            interaction: { mode: "index", intersect: false },
            scales: {
                x: {
                    type: "linear",
                    bounds: "data",
                    title: { display: true, text: "Date (UTC)" },
                    ticks: {
                        autoSkip: true,
                        maxTicksLimit: canvas.clientWidth < 480 ? 3 : 8,
                        callback: formatDate,
                    },
                },
                counts: {
                    title: {
                        display: true,
                        text: "Pages / Articles / Edits / Images",
                    },
                    ticks: { callback: formatCompact },
                },
                users: {
                    position: "right",
                    title: { display: true, text: "Users" },
                    ticks: { callback: formatCompact },
                    grid: { drawOnChartArea: false },
                },
            },
            plugins: {
                legend: { display: false },
                tooltip: {
                    callbacks: {
                        title: (items) =>
                            items.length
                                ? `${formatDateTime(items[0].parsed.x)} UTC`
                                : "",
                        label: (item) =>
                            `${item.dataset.label}: ${formatNumber(item.parsed.y)}`,
                    },
                },
            },
        },
    });

    const selectors = document.querySelectorAll("[data-chart-series]");
    const updateAxes = () => {
        chart.options.scales.counts.display = chart.data.datasets.some(
            (dataset, index) =>
                dataset.yAxisID === "counts" && chart.isDatasetVisible(index),
        );
        chart.options.scales.users.display = chart.data.datasets.some(
            (dataset, index) =>
                dataset.yAxisID === "users" && chart.isDatasetVisible(index),
        );
    };
    selectors.forEach((selector) => {
        selector.addEventListener("change", () => {
            const datasetIndex = chart.data.datasets.findIndex(
                (dataset) => dataset.field === selector.dataset.chartSeries,
            );
            if (datasetIndex === -1) return;
            chart.setDatasetVisibility(datasetIndex, selector.checked);
            updateAxes();
            chart.update();
        });
    });

    window.addEventListener("resize", () => {
        const maxTicksLimit = canvas.clientWidth < 480 ? 3 : 8;
        if (chart.options.scales.x.ticks.maxTicksLimit === maxTicksLimit) return;
        chart.options.scales.x.ticks.maxTicksLimit = maxTicksLimit;
        chart.update("none");
    });
})();
