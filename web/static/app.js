const api = "/api/v1";
const nativeApp = window.go?.desktop?.App;
const isDesktop = Boolean(nativeApp);
let financeChartInstance = null;

const state = {
  restaurantId: 0,
  restaurants: [],
  categories: [],
  excelPath: "",
  excelPreview: null,
  entries: [],
  editingEntryId: 0,
  calendarCursor: null,
  employeeNames: {},
};

const $ = (id) => document.getElementById(id);
const formData = (form) => Object.fromEntries(new FormData(form).entries());
const period = () => ({ from: $("from").value, to: $("to").value });
const periodQuery = () => new URLSearchParams(period()).toString();
const restaurantURL = (path = "") =>
  `${api}/restaurants/${state.restaurantId}${path}`;

const moneyFormatter = new Intl.NumberFormat("ru-RU", {
  style: "currency",
  currency: "RUB",
  maximumFractionDigits: 0,
});
const percentFormatter = new Intl.NumberFormat("ru-RU", {
  style: "percent",
  maximumFractionDigits: 1,
});

const money = (value) => moneyFormatter.format(Number(value || 0));
const percent = (value) => percentFormatter.format(Number(value || 0));
const escapeHTML = (value) =>
  String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");

const restaurantDependentControls = [
  "entry-form",
  "plan-form",
  "employee-form",
  "shift-form",
  "rule-form",
  "category-form",
  "pos-form",
  "excel-import",
  "payroll-export",
  "shift-calendar-toggle",
];

function setRestaurantControlsEnabled(enabled) {
  restaurantDependentControls.forEach((id) => {
    const element = $(id);
    if (!element) return;
    if (element.matches("form")) {
      Array.from(element.elements).forEach((control) => {
        control.disabled = !enabled;
      });
      return;
    }
    element.disabled = !enabled;
  });
  $("from").disabled = !enabled;
  $("to").disabled = !enabled;
  $("refresh").disabled = !enabled;
  $("export-link").classList.toggle("disabled", !enabled);
  $("export-link").setAttribute("aria-disabled", String(!enabled));
}

function kpiPercentToFraction(value) {
  const normalized = String(value ?? "").trim().replace(",", ".");
  const percentage = Number(normalized);
  if (!normalized || !Number.isFinite(percentage) || percentage < 0 || percentage > 100) {
    throw new Error("KPI должен быть числом от 0 до 100 процентов");
  }
  return String(percentage / 100);
}

async function request(url, options = {}) {
  const response = await fetch(url, options);
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${response.status}`);
  }
  return response.headers.get("content-type")?.includes("json")
    ? response.json()
    : response;
}

const json = (body) => ({
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify(body),
});

const backend = {
  restaurants: () =>
    isDesktop ? nativeApp.Restaurants() : request(`${api}/restaurants`),

  createRestaurant: (value) =>
    isDesktop
      ? nativeApp.CreateRestaurant(value)
      : request(`${api}/restaurants`, { method: "POST", ...json(value) }),

  categories: (restaurantId) =>
    isDesktop
      ? nativeApp.Categories(restaurantId)
      : request(`${api}/restaurants/${restaurantId}/categories`),

  createCategory: (restaurantId, value) =>
    isDesktop
      ? nativeApp.CreateCategory(restaurantId, value)
      : request(`${api}/restaurants/${restaurantId}/categories`, {
          method: "POST",
          ...json(value),
        }),

  dashboardFor: (restaurantId, selectedPeriod) =>
    isDesktop
      ? nativeApp.Dashboard(restaurantId, selectedPeriod)
      : request(
          `${api}/restaurants/${restaurantId}/reports/dashboard?${new URLSearchParams(selectedPeriod).toString()}`,
        ),

  dashboard: (restaurantId) => backend.dashboardFor(restaurantId, period()),

  entries: (restaurantId) =>
    isDesktop
      ? nativeApp.Entries(restaurantId, period(), {
          limit: 200,
          offset: 0,
          direction: "",
          query: "",
        })
      : request(
          `${api}/restaurants/${restaurantId}/entries?${periodQuery()}&limit=200`,
        ),

  createEntry: (restaurantId, value) =>
    isDesktop
      ? nativeApp.CreateEntry(restaurantId, value)
      : request(`${api}/restaurants/${restaurantId}/entries`, {
          method: "POST",
          ...json(value),
        }),

  updateEntry: (restaurantId, entryId, value) =>
    isDesktop
      ? nativeApp.UpdateEntry(restaurantId, entryId, value)
      : request(`${api}/restaurants/${restaurantId}/entries/${entryId}`, {
          method: "PUT",
          ...json(value),
        }),

  savePlan: (restaurantId, value) =>
    isDesktop
      ? nativeApp.SavePlan(restaurantId, value)
      : request(`${api}/restaurants/${restaurantId}/plans`, {
          method: "POST",
          ...json(value),
        }),

  rules: (restaurantId) =>
    isDesktop
      ? nativeApp.Rules(restaurantId)
      : request(`${api}/restaurants/${restaurantId}/rules`),

  createRule: (restaurantId, value) =>
    isDesktop
      ? nativeApp.CreateRule(restaurantId, value)
      : request(`${api}/restaurants/${restaurantId}/rules`, {
          method: "POST",
          ...json(value),
        }),

  employees: (restaurantId) =>
    isDesktop
      ? nativeApp.Employees(restaurantId)
      : request(`${api}/restaurants/${restaurantId}/employees`),

  inactiveEmployees: (restaurantId) =>
    isDesktop
      ? nativeApp.InactiveEmployees(restaurantId)
      : request(`${api}/restaurants/${restaurantId}/employees?include_inactive=true`)
        .then((employees) => employees.filter((employee) => !employee.active)),

  saveEmployee: (restaurantId, value) =>
    isDesktop
      ? nativeApp.SaveEmployee(restaurantId, value)
      : request(`${api}/restaurants/${restaurantId}/employees`, {
          method: "POST",
          ...json(value),
        }),

  deleteEmployee: (restaurantId, employeeId) =>
    isDesktop
      ? nativeApp.DeleteEmployee(restaurantId, employeeId)
      : request(`${api}/restaurants/${restaurantId}/employees/${employeeId}`, {
          method: "DELETE",
        }),

  exportPayroll: (restaurantId, selectedPeriod) =>
    isDesktop
      ? nativeApp.ExportPayrollExcel(restaurantId, selectedPeriod)
      : `${restaurantURL("/reports/payroll/export")}?${new URLSearchParams(selectedPeriod).toString()}`,

  saveShift: (restaurantId, value) =>
    isDesktop
      ? nativeApp.SaveShift(restaurantId, value)
      : request(`${api}/restaurants/${restaurantId}/shifts`, {
          method: "POST",
          ...json(value),
        }),

  shifts: (restaurantId, selectedPeriod) =>
    isDesktop
      ? nativeApp.Shifts(restaurantId, selectedPeriod)
      : request(
          `${api}/restaurants/${restaurantId}/shifts?${new URLSearchParams(selectedPeriod).toString()}`,
        ),

  posConnections: (restaurantId) =>
    isDesktop
      ? nativeApp.POSConnections(restaurantId)
      : request(`${api}/restaurants/${restaurantId}/pos-connections`),

  savePOSConnection: (restaurantId, value) =>
    isDesktop
      ? nativeApp.SavePOSConnection(restaurantId, value)
      : request(`${api}/restaurants/${restaurantId}/pos-connections`, {
          method: "POST",
          ...json(value),
        }),

  testPOS: (restaurantId, connectionId) =>
    isDesktop
      ? nativeApp.TestPOS(restaurantId, connectionId)
      : request(
          `${api}/restaurants/${restaurantId}/pos-connections/${connectionId}/test`,
          { method: "POST" },
        ),

  syncPOS: (restaurantId, connectionId) =>
    isDesktop
      ? nativeApp.SyncPOS(restaurantId, connectionId, period())
      : request(
          `${api}/restaurants/${restaurantId}/pos-connections/${connectionId}/sync?${periodQuery()}`,
          { method: "POST" },
        ),
};

function toast(message) {
  $("toast").textContent =
    typeof message === "string" ? message : message?.message || String(message);
  $("toast").classList.add("show");
  setTimeout(() => $("toast").classList.remove("show"), 3000);
}

function setDefaultDates() {
  const now = new Date();
  const firstDay = new Date(now.getFullYear(), now.getMonth(), 1);
  const from = firstDay.toISOString().slice(0, 10);
  const to = now.toISOString().slice(0, 10);

  $("from").value = from;
  $("to").value = to;
  $("template-month").value = from.slice(0, 7);
  $("plan-month").value = from.slice(0, 7);
  document.querySelector('#entry-form [name="date"]').value = to;
  document.querySelector('#shift-form [name="date"]').value = to;
  state.calendarCursor = new Date(now.getFullYear(), now.getMonth(), 1);
}

function dateKey(date) {
  const pad = (value) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

function calendarPeriod() {
  const cursor = state.calendarCursor || new Date();
  return {
    from: dateKey(new Date(cursor.getFullYear(), cursor.getMonth(), 1)),
    to: dateKey(new Date(cursor.getFullYear(), cursor.getMonth() + 1, 0)),
  };
}

function previousPeriod() {
  const from = new Date(`${$("from").value}T00:00:00`);
  const to = new Date(`${$("to").value}T00:00:00`);
  if (Number.isNaN(from.getTime()) || Number.isNaN(to.getTime()) || to < from) return null;
  const days = Math.round((to - from) / 86400000) + 1;
  const previousTo = new Date(from.getTime() - 86400000);
  const previousFrom = new Date(previousTo.getTime() - (days - 1) * 86400000);
  const format = (date) => {
    const pad = (value) => String(value).padStart(2, "0");
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  };
  return { from: format(previousFrom), to: format(previousTo) };
}

async function loadRestaurants() {
  state.restaurants = await backend.restaurants();
  $("restaurant").innerHTML = state.restaurants.length
    ? state.restaurants
        .map(({ id, name }) => `<option value="${id}">${escapeHTML(name)}</option>`)
        .join("")
    : '<option value="">Сначала создайте ресторан</option>';

  if (state.restaurants.length) {
    await changeRestaurant();
    return;
  }
  state.restaurantId = 0;
  setRestaurantControlsEnabled(false);
  $("status").textContent = "Сначала создайте ресторан в настройках";
}

async function changeRestaurant() {
  state.restaurantId = Number($("restaurant").value || 0);
  setRestaurantControlsEnabled(Boolean(state.restaurantId));
  if (!state.restaurantId) {
    $("status").textContent = "Сначала выберите или создайте ресторан";
    return;
  }

  state.categories = await backend.categories(state.restaurantId);
  const options = state.categories
    .map(({ id, name }) => `<option value="${id}">${escapeHTML(name)}</option>`)
    .join("");

  $("entry-category").innerHTML =
    '<option value="">Определить правилом</option>' + options;
  $("rule-category").innerHTML = options;
  $("plan-category").innerHTML = options;
  await refresh();
}

async function refresh() {
  if (!state.restaurantId) return;

  const previous = previousPeriod();
  const [dashboard, entries, rules, connections, employees, inactiveEmployees, previousDashboard, shifts] = await Promise.all([
    backend.dashboard(state.restaurantId),
    backend.entries(state.restaurantId),
    backend.rules(state.restaurantId),
    backend.posConnections(state.restaurantId),
    backend.employees(state.restaurantId),
    backend.inactiveEmployees(state.restaurantId),
    previous ? backend.dashboardFor(state.restaurantId, previous) : Promise.resolve(null),
    backend.shifts(state.restaurantId, calendarPeriod()),
  ]);

  state.entries = entries;
  state.employeeNames = Object.fromEntries(
    [...employees, ...inactiveEmployees].map((employee) => [Number(employee.id), employee.name]),
  );
  renderKPIs(dashboard);
  renderComparison(dashboard, previousDashboard, previous);
  renderPnL(dashboard.pnl);
  renderEntries(entries);
  renderRules(rules);
  renderConnections(connections);
  renderPayroll(employees, dashboard.payroll);
  renderEmployeeLists(employees, inactiveEmployees);
  renderShiftCalendar(shifts);
  renderFinanceCharts(dashboard, entries);

  if (!isDesktop) {
    $("export-link").href = restaurantURL(
      `/excel/export?${periodQuery()}`,
    );
  }
  $("status").textContent = `Данные за ${$("from").value} — ${$("to").value}`;
}

function renderShiftCalendar(shifts) {
  const cursor = state.calendarCursor || new Date();
  const year = cursor.getFullYear();
  const month = cursor.getMonth();
  const firstWeekday = (new Date(year, month, 1).getDay() + 6) % 7;
  const daysInMonth = new Date(year, month + 1, 0).getDate();
  const today = dateKey(new Date());
  const byDate = new Map();

  (shifts || []).forEach((shift) => {
    const key = String(shift.date || "").slice(0, 10);
    const values = byDate.get(key) || [];
    values.push(shift);
    byDate.set(key, values);
  });

  $("calendar-month").textContent = new Intl.DateTimeFormat("ru-RU", {
    month: "long",
    year: "numeric",
  }).format(cursor);

  const blanks = Array.from({ length: firstWeekday }, () => '<span class="calendar-day empty"></span>');
  const days = Array.from({ length: daysInMonth }, (_, index) => {
    const day = index + 1;
    const key = dateKey(new Date(year, month, day));
    const dayShifts = byDate.get(key) || [];
    const names = dayShifts.map((shift) => state.employeeNames[Number(shift.employee_id)] || "Сотрудник");
    const title = dayShifts.length
      ? `${dayShifts.length} смен: ${names.join(", ")}`
      : "Смен нет";
    return `<span class="calendar-day${dayShifts.length ? " has-shifts" : ""}${key === today ? " today" : ""}" title="${escapeHTML(title)}">
      ${day}${dayShifts.length ? `<small>${dayShifts.length}</small>` : ""}
    </span>`;
  });
  $("shift-calendar-grid").innerHTML = [...blanks, ...days].join("");
  $("calendar-summary").textContent = shifts?.length
    ? `Всего смен за месяц: ${shifts.length}`
    : "В этом месяце смен пока нет";
}

async function moveCalendar(monthDelta) {
  const cursor = state.calendarCursor || new Date();
  state.calendarCursor = new Date(cursor.getFullYear(), cursor.getMonth() + monthDelta, 1);
  if (!state.restaurantId) {
    renderShiftCalendar([]);
    return;
  }
  renderShiftCalendar(await backend.shifts(state.restaurantId, calendarPeriod()));
}

function renderComparison(current, previous, previousRange) {
  if (!previous || !previousRange) {
    $("comparison-periods").textContent = "Недоступно для выбранного периода";
    ["comparison-revenue", "comparison-profit", "comparison-cash"].forEach((id) => { $(id).textContent = "—"; });
    return;
  }
  const values = [
    ["comparison-revenue", current.pnl.revenue, previous.pnl.revenue],
    ["comparison-profit", current.pnl.operating_profit, previous.pnl.operating_profit],
    ["comparison-cash", current.cash_flow.net_cash_flow, previous.cash_flow.net_cash_flow],
  ];
  values.forEach(([id, currentValue, previousValue]) => {
    const now = Number(currentValue || 0);
    const before = Number(previousValue || 0);
    const delta = now - before;
    const percentDelta = before ? (delta / Math.abs(before)) * 100 : null;
    $(id).textContent = `${money(now)} · ${delta >= 0 ? "+" : ""}${money(delta)}${percentDelta === null ? "" : ` (${percentDelta >= 0 ? "+" : ""}${percentDelta.toFixed(1)}%)`}`;
  });
  $("comparison-periods").textContent = `${$("from").value} — ${$("to").value} против ${previousRange.from} — ${previousRange.to}`;
}

function renderKPIs(dashboard) {
  const { pnl, cash_flow: cashFlow } = dashboard;
  $("kpi-revenue").textContent = money(pnl.revenue);
  $("kpi-gross").textContent = money(pnl.gross_profit);
  $("kpi-ebitda").textContent = money(pnl.ebitda);
  $("kpi-profit").textContent = money(pnl.operating_profit);
  $("kpi-margin").textContent = `Маржа ${percent(pnl.margin)}`;
  $("kpi-gross-margin").textContent = Number(pnl.revenue)
    ? `Маржа ${percent(Number(pnl.gross_profit) / Number(pnl.revenue))}`
    : "—";
  $("kpi-break-even").textContent = money(dashboard.break_even_revenue);
  $("kpi-cash").textContent = money(cashFlow.net_cash_flow);
}

function renderPnL(pnl) {
  $("pnl-lines").innerHTML = (pnl.lines || [])
    .map(
      (line) => `<tr>
        <td>${escapeHTML(line.name)}${line.calculated ? " · формула" : ""}</td>
        <td>${money(line.actual)}</td>
        <td>${money(line.plan)}</td>
        <td>${money(line.variance)}</td>
        <td>${percent(line.percent)}</td>
      </tr>`,
    )
    .join("");
}

function categoryNames() {
  return Object.fromEntries(
    state.categories.map((category) => [category.id, category.name]),
  );
}

function renderEntries(entries) {
  const names = categoryNames();
  $("entry-lines").innerHTML = entries
    .map(
      (entry) => `<tr>
        <td>${String(entry.occurred_at).slice(0, 10)}</td>
        <td>${escapeHTML(entry.description || "—")}</td>
        <td>${escapeHTML(names[entry.category_id] || "Не определена")}</td>
        <td>${entry.direction === "income" ? "Доход" : "Расход"}</td>
        <td>${escapeHTML(entry.tags || "—")}</td>
        <td>${money(entry.amount)}</td>
        <td class="entry-actions">
          <button class="secondary" data-entry-edit="${entry.id}">Изменить</button>
          <button class="secondary" data-entry-duplicate="${entry.id}">Дублировать</button>
        </td>
      </tr>`,
    )
    .join("");
}

function renderEmployeeLists(active, inactive) {
  $("active-employee-list").innerHTML = active.length
    ? active.map((employee) => `<li>${escapeHTML(employee.name)} <small>${escapeHTML(employee.position)}</small></li>`).join("")
    : "<li>Нет активных сотрудников</li>";
  $("inactive-employee-list").innerHTML = inactive.length
    ? inactive.map((employee) => `<li>${escapeHTML(employee.name)} <small>${escapeHTML(employee.position)}</small></li>`).join("")
    : "<li>Нет уволенных сотрудников</li>";
}

function resetEntryForm() {
  state.editingEntryId = 0;
  $("entry-form").reset();
  $("entry-form-title").textContent = "Новая операция";
  $("entry-submit").textContent = "Сохранить операцию";
  $("entry-cancel").hidden = true;
}

function editEntry(entry) {
  state.editingEntryId = Number(entry.id);
  const form = $("entry-form");
  form.elements.date.value = String(entry.occurred_at || "").slice(0, 10);
  form.elements.direction.value = entry.direction;
  form.elements.amount.value = entry.amount;
  form.elements.category_id.value = entry.category_id || "";
  form.elements.payment_method.value = entry.payment_method || "other";
  form.elements.description.value = entry.description || "";
  form.elements.counterparty.value = entry.counterparty || "";
  form.elements.tags.value = entry.tags || "";
  $("entry-form-title").textContent = "Редактирование операции";
  $("entry-submit").textContent = "Сохранить изменения";
  $("entry-cancel").hidden = false;
  document.querySelector('.nav[data-page="entries"]').click();
}

async function duplicateEntry(entry) {
  const value = {
    date: String(entry.occurred_at || "").slice(0, 10),
    direction: entry.direction,
    amount: String(entry.amount),
    category_id: entry.category_id || undefined,
    payment_method: entry.payment_method || "other",
    description: entry.description || "",
    counterparty: entry.counterparty || "",
    tags: entry.tags || "",
  };
  await backend.createEntry(state.restaurantId, value);
  toast("Операция продублирована");
  await refresh();
}

function renderRules(rules) {
  const names = categoryNames();
  $("rule-list").innerHTML =
    rules
      .map((rule) => {
        const formula =
          rule.rule_type === "classification"
            ? `${rule.match_field} ${rule.match_operator} “${rule.match_value}”`
            : `${rule.source_metric || "статья"} × ${rule.rate} + ${rule.fixed_amount}`;
        return `<div class="rule-item">
          <strong>${escapeHTML(rule.name)}</strong>
          <small>${escapeHTML(formula)} → ${escapeHTML(names[rule.target_category_id] || "статья")}</small>
        </div>`;
      })
      .join("") || "<p>Правил пока нет</p>";
}

function renderConnections(connections) {
  $("pos-list").innerHTML =
    connections
      .map(
        (connection) => `<div class="rule-item">
          <strong>${escapeHTML(connection.name)}</strong>
          <small>${escapeHTML(connection.provider)} · ${escapeHTML(connection.base_url)}</small>
          <button data-pos-test="${connection.id}">Проверить</button>
          <button data-pos-sync="${connection.id}">Синхронизировать</button>
        </div>`,
      )
      .join("") || "<p>Подключений пока нет</p>";
}

function renderPayroll(employees, payroll) {
  const activeEmployeeIDs = new Set(employees.map(({ id }) => Number(id)));
  $("shift-employee").innerHTML = employees
    .map(
      (employee) =>
        `<option value="${employee.id}">${escapeHTML(employee.name)} · ${escapeHTML(employee.position)}</option>`,
    )
    .join("");
  $("payroll-lines").innerHTML = (payroll.lines || [])
    .map(
      (line) => {
        const action = activeEmployeeIDs.has(Number(line.employee_id))
          ? `<button class="danger" data-employee-delete="${line.employee_id}" data-employee-name="${escapeHTML(line.name)}">Уволить</button>`
          : '<span class="employee-inactive">Уволен</span>';
        return `<tr>
        <td>${escapeHTML(line.name)}</td>
        <td>${Number(line.shift_count || 0)}</td>
        <td>${line.hours}</td>
        <td>${money(line.gross)}</td>
        <td>${money(line.kpi)}</td>
        <td>${money(line.net)}</td>
        <td>${action}</td>
      </tr>`;
      },
    )
    .join("");
}

async function syncPOS(id) {
  const result = await backend.syncPOS(state.restaurantId, Number(id));
  toast(`Импортировано: ${result.imported}`);
  await refresh();
}

async function testPOS(id) {
  await backend.testPOS(state.restaurantId, Number(id));
  toast("Подключение работает");
}

async function exportPayroll() {
  const result = await backend.exportPayroll(state.restaurantId, period());
  if (isDesktop) {
    if (result) toast(`Расчёт сохранён: ${result}`);
    return;
  }
  window.open(result, "_blank");
}

function excelColumnName(index) {
  let name = "";
  for (let value = index + 1; value > 0; value = Math.floor((value - 1) / 26)) {
    name = String.fromCharCode(65 + ((value - 1) % 26)) + name;
  }
  return name;
}

function renderExcelSheet(sheetName) {
  const sheet = state.excelPreview?.sheets?.find(({ name }) => name === sheetName);
  const previewRows = Array.isArray(sheet?.rows)
    ? sheet.rows.map((row) => (Array.isArray(row) ? row : []))
    : [];
  if (!sheet || !previewRows.length) {
    $("excel-output").innerHTML = '<p class="excel-empty">На выбранном листе нет данных</p>';
    return;
  }

  const totalColumns = Math.max(...previewRows.map((row) => row.length), 1);
  const visibleColumns = Math.min(totalColumns, 50);
  const columnHeaders = Array.from(
    { length: visibleColumns },
    (_, index) => `<th>${excelColumnName(index)}</th>`,
  ).join("");
  const rows = previewRows
    .map((row, rowIndex) => {
      const cells = Array.from(
        { length: visibleColumns },
        (_, columnIndex) => `<td title="${escapeHTML(row[columnIndex] || "")}">${escapeHTML(row[columnIndex] || "")}</td>`,
      ).join("");
      return `<tr><th class="excel-row-number">${rowIndex + 1}</th>${cells}</tr>`;
    })
    .join("");
  const limitNote = totalColumns > visibleColumns
    ? ` · показаны первые ${visibleColumns} из ${totalColumns} колонок`
    : "";

  $("excel-output").innerHTML = `
    <div class="excel-preview-meta">
      <span>Лист: ${escapeHTML(sheet.name)}</span>
      <span>${previewRows.length} строк${limitNote}</span>
    </div>
    <div class="excel-table-wrap">
      <table class="excel-table">
        <thead><tr><th class="excel-row-number">№</th>${columnHeaders}</tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>`;
}

function renderExcelPreview(result) {
  state.excelPreview = result;
  const sheets = Array.isArray(result?.sheets) ? result.sheets : [];
  $("excel-sheet").innerHTML = sheets
    .map(({ name }) => `<option value="${escapeHTML(name)}">${escapeHTML(name)}</option>`)
    .join("");
  if (!sheets.length) {
    $("excel-output").innerHTML = '<p class="excel-empty">В файле не найдено листов</p>';
    return;
  }
  renderExcelSheet(sheets[0].name);
}

function renderExcelImportResult(result) {
  const entries = result?.entries || [];
  const errors = result?.errors || [];
  const visibleEntries = entries.slice(0, 100);
  const rows = visibleEntries
    .map((entry, index) => `<tr>
      <th class="excel-row-number">${index + 1}</th>
      <td>${escapeHTML(String(entry.occurred_at || "").slice(0, 10))}</td>
      <td title="${escapeHTML(entry.description || "")}">${escapeHTML(entry.description || "—")}</td>
      <td>${entry.direction === "income" ? "Доход" : "Расход"}</td>
      <td>${money(entry.amount)}</td>
    </tr>`)
    .join("");
  const table = visibleEntries.length
    ? `<div class="excel-table-wrap">
        <table class="excel-table">
          <thead><tr><th class="excel-row-number">№</th><th>Дата</th><th>Описание</th><th>Тип</th><th>Сумма</th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>`
    : '<p class="excel-empty">Распознанных операций нет</p>';
  const errorList = errors.length
    ? `<ul class="excel-errors">${errors.map((error) => `<li>${escapeHTML(error)}</li>`).join("")}</ul>`
    : "";

  $("excel-output").innerHTML = `
    <div class="excel-result-cards">
      <div class="excel-result-card"><span>Импортировано</span><strong>${Number(result?.imported || 0)}</strong></div>
      <div class="excel-result-card"><span>Пропущено</span><strong>${Number(result?.skipped || 0)}</strong></div>
      <div class="excel-result-card"><span>Ошибки</span><strong>${errors.length}</strong></div>
    </div>
    ${table}
    ${errorList}`;
}

async function previewExcel() {
  if (isDesktop) {
    const selection = await nativeApp.SelectExcel();
    if (!selection.path) return;
    state.excelPath = selection.path;
    renderExcelPreview(selection.preview);
    return;
  }

  const file = $("excel-file").files[0];
  if (!file) throw new Error("Выберите Excel-файл");
  const body = new FormData();
  body.append("file", file);
  renderExcelPreview(
    await request(`${api}/excel/preview`, { method: "POST", body }),
  );
}

function excelMapping() {
  return {
    sheet: $("excel-sheet").value,
    has_header: $("map-header").checked,
    date: Number($("map-date").value),
    amount: Number($("map-amount").value),
    direction: Number($("map-direction").value),
    description: Number($("map-description").value),
    counterparty: Number($("map-counterparty").value),
    payment_method: Number($("map-payment").value),
    category_code: Number($("map-category").value),
  };
}

async function importExcel() {
  if (!state.restaurantId) {
    throw new Error("Сначала выберите или создайте ресторан для импорта");
  }
  let result;
  if (isDesktop) {
    if (!state.excelPath) throw new Error("Сначала выберите Excel-файл");
    result = await nativeApp.ImportExcel({
      path: state.excelPath,
      restaurant_id: state.restaurantId,
      template_mode: $("template-mode").checked,
      month: $("template-month").value,
      mapping: excelMapping(),
    });
  } else {
    const file = $("excel-file").files[0];
    if (!file) throw new Error("Выберите Excel-файл");
    const body = new FormData();
    body.append("file", file);
    if ($("template-mode").checked) {
      body.append("mode", "template");
      body.append("month", $("template-month").value);
    } else {
      body.append("mapping", JSON.stringify(excelMapping()));
    }
    result = await request(restaurantURL("/excel/import"), {
      method: "POST",
      body,
    });
  }

  renderExcelImportResult(result);
  toast(`Импортировано: ${result.imported}, пропущено: ${result.skipped}`);
  await refresh();
}

async function exportExcel(event) {
  if (!isDesktop) return;
  event.preventDefault();
  const path = await nativeApp.ExportExcel(state.restaurantId, period());
  if (path) toast(`Отчёт сохранён: ${path}`);
}

function handle(action) {
  return async (event) => {
    event?.preventDefault();
    try {
      await action(event);
    } catch (error) {
      toast(error);
    }
  };
}

document.querySelectorAll(".nav").forEach((button) => {
  button.addEventListener("click", () => {
    document
      .querySelectorAll(".nav,.page")
      .forEach((element) => element.classList.remove("active"));
    button.classList.add("active");
    $(button.dataset.page).classList.add("active");
    $("page-title").textContent = button.textContent;
  });
});

$("restaurant").addEventListener("change", handle(changeRestaurant));
$("refresh").addEventListener("click", handle(refresh));
$("payroll-export").addEventListener("click", handle(exportPayroll));
$("shift-calendar-toggle").addEventListener("click", () => {
  const panel = $("shift-calendar-panel");
  panel.hidden = !panel.hidden;
  $("shift-calendar-toggle").setAttribute("aria-expanded", String(!panel.hidden));
});
$("calendar-prev").addEventListener("click", handle(() => moveCalendar(-1)));
$("calendar-next").addEventListener("click", handle(() => moveCalendar(1)));
document.addEventListener("click", (event) => {
  if (event.target.closest(".shift-calendar")) return;
  $("shift-calendar-panel").hidden = true;
  $("shift-calendar-toggle").setAttribute("aria-expanded", "false");
});
if (isDesktop) {
  $("export-link").addEventListener("click", handle(exportExcel));
}
$("rule-type").addEventListener("change", (event) => {
  const calculation = event.target.value === "calculation";
  $("classification-fields").hidden = calculation;
  $("calculation-fields").hidden = !calculation;
});
$("pos-list").addEventListener(
  "click",
  handle(async (event) => {
    const testId = event.target.dataset.posTest;
    const syncId = event.target.dataset.posSync;
    if (testId) await testPOS(testId);
    if (syncId) await syncPOS(syncId);
  }),
);

$("entry-form").addEventListener(
  "submit",
  handle(async (event) => {
    const value = formData(event.target);
    if (value.category_id) value.category_id = Number(value.category_id);
    else delete value.category_id;
    if (state.editingEntryId) {
      await backend.updateEntry(state.restaurantId, state.editingEntryId, value);
      toast("Операция изменена");
    } else {
      await backend.createEntry(state.restaurantId, value);
      toast("Операция сохранена");
    }
    resetEntryForm();
    $("entry-form").elements.date.value = $("to").value;
    await refresh();
  }),
);

$("entry-cancel").addEventListener("click", resetEntryForm);
$("entry-lines").addEventListener(
  "click",
  handle(async (event) => {
    const editButton = event.target.closest("[data-entry-edit]");
    const duplicateButton = event.target.closest("[data-entry-duplicate]");
    if (editButton) {
      const entry = state.entries.find(({ id }) => Number(id) === Number(editButton.dataset.entryEdit));
      if (entry) editEntry(entry);
    }
    if (duplicateButton) {
      const entry = state.entries.find(({ id }) => Number(id) === Number(duplicateButton.dataset.entryDuplicate));
      if (entry) await duplicateEntry(entry);
    }
  }),
);

$("plan-form").addEventListener(
  "submit",
  handle(async (event) => {
    const value = formData(event.target);
    value.category_id = Number(value.category_id);
    await backend.savePlan(state.restaurantId, value);
    toast("План сохранён");
    await refresh();
  }),
);

$("employee-form").addEventListener(
  "submit",
  handle(async (event) => {
    if (!state.restaurantId) {
      throw new Error("Сначала выберите или создайте ресторан");
    }
    const value = formData(event.target);
    value.kpi_percent = kpiPercentToFraction(value.kpi_percent);
    await backend.saveEmployee(state.restaurantId, {
      ...value,
      active: true,
    });
    toast("Сотрудник сохранён");
    event.target.reset();
    await refresh();
  }),
);

$("shift-form").addEventListener(
  "submit",
  handle(async (event) => {
    const value = formData(event.target);
    value.employee_id = Number(value.employee_id);
    await backend.saveShift(state.restaurantId, value);
    toast("Смена добавлена");
    await refresh();
  }),
);

$("payroll-lines").addEventListener(
  "click",
  handle(async (event) => {
    const button = event.target.closest("[data-employee-delete]");
    if (!button) return;
    const name = button.dataset.employeeName || "сотрудника";
    if (!window.confirm(`Уволить сотрудника «${name}»? История его смен сохранится.`)) {
      return;
    }
    await backend.deleteEmployee(
      state.restaurantId,
      Number(button.dataset.employeeDelete),
    );
    toast("Сотрудник удалён из активного списка");
    await refresh();
  }),
);

$("rule-form").addEventListener(
  "submit",
  handle(async (event) => {
    const value = formData(event.target);
    value.priority = Number(value.priority);
    value.target_category_id = Number(value.target_category_id);
    value.rate ||= "0";
    value.fixed_amount ||= "0";
    if (value.rule_type === "classification") {
      delete value.source_metric;
      delete value.operation;
    } else {
      delete value.match_field;
      delete value.match_operator;
      delete value.match_value;
    }
    await backend.createRule(state.restaurantId, value);
    toast("Правило добавлено");
    await refresh();
  }),
);

$("restaurant-form").addEventListener(
  "submit",
  handle(async (event) => {
    await backend.createRestaurant(formData(event.target));
    toast("Ресторан создан");
    await loadRestaurants();
  }),
);

$("category-form").addEventListener(
  "submit",
  handle(async (event) => {
    await backend.createCategory(
      state.restaurantId,
      formData(event.target),
    );
    toast("Статья добавлена");
    await changeRestaurant();
  }),
);

$("pos-form").addEventListener(
  "submit",
  handle(async (event) => {
    await backend.savePOSConnection(state.restaurantId, {
      ...formData(event.target),
      active: true,
    });
    toast("Подключение сохранено");
    await refresh();
  }),
);

$("excel-preview").addEventListener("click", handle(previewExcel));
$("excel-import").addEventListener("click", handle(importExcel));
$("excel-sheet").addEventListener("change", () => {
  renderExcelSheet($("excel-sheet").value);
});

if (isDesktop) {
  $("excel-file").hidden = true;
}
// --- ЛОГИКА ГРАФИКОВ ---
function renderFinanceCharts(dashboard, entries) {
  const ctx = document.getElementById('financeChart')?.getContext('2d');
  if (!ctx) return;

  if (financeChartInstance) {
    financeChartInstance.destroy();
  }

  const intervalMode = document.getElementById('chart-interval')?.value || 'day';

  if (intervalMode === 'day') {
    const dailyData = {};
    (entries || []).forEach(entry => {
      const day = String(entry.occurred_at || '').slice(0, 10);
      if (!dailyData[day]) dailyData[day] = { income: 0, expense: 0 };
      const amount = Number(entry.amount || 0);
      if (entry.direction === 'income') dailyData[day].income += amount;
      else dailyData[day].expense += amount;
    });

    const sortedDays = Object.keys(dailyData).sort();
    const incomes = sortedDays.map(day => dailyData[day].income);
    const expenses = sortedDays.map(day => dailyData[day].expense);

    financeChartInstance = new Chart(ctx, {
      type: 'line',
      data: {
        labels: sortedDays,
        datasets: [
          {
            label: 'Доходы (Выручка)',
            data: incomes,
            borderColor: '#183f42',
            backgroundColor: 'rgba(24, 63, 66, 0.1)',
            tension: 0.3,
            fill: true,
          },
          {
            label: 'Расходы',
            data: expenses,
            borderColor: '#9b2f26',
            backgroundColor: 'rgba(155, 47, 38, 0.1)',
            tension: 0.3,
            fill: true,
          }
        ]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: { legend: { position: 'bottom' } }
      }
    });
  } else {
    const pnlLines = dashboard.pnl?.lines || [];
    const labels = pnlLines.map(l => l.name);
    const data = pnlLines.map(l => Number(l.actual || 0));

    financeChartInstance = new Chart(ctx, {
      type: 'bar',
      data: {
        labels: labels,
        datasets: [{
          label: 'Фактическая сумма по статьям',
          data: data,
          backgroundColor: '#ddff63',
          borderColor: '#183f42',
          borderWidth: 1
        }]
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: { legend: { display: false } }
      }
    });
  }
}

// Слушатель для переключателя режимов графика
document.getElementById('chart-interval')?.addEventListener('change', () => {
  if (state.restaurantId) refresh();
});
// --- КОНЕЦ ЛОГИКИ ГРАФИКОВ ---
setDefaultDates();
setRestaurantControlsEnabled(false);
loadRestaurants().catch(toast);
