import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

const MENU = [
  { key: "dashboard", label: "工作台", hint: "运行概览" },
  { key: "providers", label: "第三方渠道商", hint: "渠道与映射" },
  { key: "logs", label: "请求日志", hint: "调用追踪" },
  { key: "users", label: "用户管理", hint: "后台账号" },
];

const PROVIDER_EMPTY = { code: "", name: "", type: "", base_url: "", api_key: "", timeout_seconds: 180, enabled: true };
const USER_EMPTY = { username: "", password: "", role: "admin", enabled: true };
const MAPPING_EMPTY = {
  name: "",
  public_model: "gpt-image-2",
  target_protocol: "openai",
  target_endpoint: "openai.images.generations",
  provider_code: "",
  upstream_model: "canvas-20",
  upstream_model_field: "model",
  upstream_method: "POST",
  upstream_path: "/v1/content/models/canvas-20/generations",
  body_mode: "json",
  field_map_json: "{}",
  file_field_map_json: "{}",
  defaults_json: "{}",
  ignore_fields_json: "[]",
  header_map_json: "{}",
  response_field_map_json: "{}",
  response_defaults_json: "{}",
  error_field_map_json: "{}",
  error_defaults_json: "{}",
  normalize_openai_usage: true,
  enabled: true,
};

function apiClient(token, onUnauthorized) {
  async function request(path, options = {}) {
    const res = await fetch(path, {
      ...options,
      headers: {
        ...(options.headers || {}),
        Authorization: token ? `Bearer ${token}` : "",
      },
    });
    const text = await res.text();
    const body = text ? JSON.parse(text) : {};
    if (res.status === 401) {
      onUnauthorized?.();
      throw new Error("登录已过期，请重新登录");
    }
    if (!res.ok) throw new Error(body.message || body.error?.message || "请求失败");
    return body;
  }
  return {
    get: (path) => request(path),
    json: (path, method, data) => request(path, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    }),
  };
}

function App() {
  const [token, setToken] = useState(localStorage.getItem("gateway_token") || "");
  const [route, setRoute] = useState({ page: "dashboard", params: {} });
  const api = useMemo(() => apiClient(token, logout), [token]);

  function logout() {
    localStorage.removeItem("gateway_token");
    setToken("");
  }

  function navigate(page, params = {}) {
    setRoute({ page, params });
  }

  if (!token) return <Login onLogin={setToken} />;

  const activeMenu = route.page.startsWith("provider") || route.page.startsWith("mapping") ? "providers" : route.page;

  return (
    <div className="app-shell">
      <aside className="app-sidebar">
        <div className="brand-block">
          <div className="brand-mark">CA</div>
          <div>
            <strong>渠道适配网关</strong>
            <span>Channel Adapter</span>
          </div>
        </div>
        <nav className="side-menu">
          {MENU.map((item) => (
            <button key={item.key} className={activeMenu === item.key ? "active" : ""} onClick={() => navigate(item.key)}>
              <span>{item.label}</span>
              <small>{item.hint}</small>
            </button>
          ))}
        </nav>
        <button className="logout-button" onClick={logout}>退出登录</button>
      </aside>
      <main className="app-main">
        {route.page === "dashboard" && <Dashboard api={api} navigate={navigate} />}
        {route.page === "providers" && <ProviderListPage api={api} navigate={navigate} />}
        {route.page === "provider-create" && <ProviderFormPage api={api} navigate={navigate} />}
        {route.page === "provider-edit" && <ProviderFormPage api={api} navigate={navigate} providerId={route.params.id} />}
        {route.page === "provider-detail" && <ProviderDetailPage api={api} navigate={navigate} providerCode={route.params.code} />}
        {route.page === "mapping-create" && <MappingFormPage api={api} navigate={navigate} providerCode={route.params.providerCode} />}
        {route.page === "mapping-edit" && <MappingFormPage api={api} navigate={navigate} mappingId={route.params.id} providerCode={route.params.providerCode} />}
        {route.page === "logs" && <LogsPage api={api} />}
        {route.page === "users" && <UsersPage api={api} />}
      </main>
    </div>
  );
}

function Login({ onLogin }) {
  const [form, setForm] = useState({ username: "admin", password: "" });
  const [message, setMessage] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(e) {
    e.preventDefault();
    setMessage("");
    setSubmitting(true);
    try {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });
      const body = await res.json();
      if (!res.ok) throw new Error(body.message || "登录失败");
      localStorage.setItem("gateway_token", body.token);
      onLogin(body.token);
    } catch (err) {
      setMessage(err.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="login-page">
      <form className="login-card" onSubmit={submit}>
        <div className="login-logo">CA</div>
        <h1>渠道适配网关</h1>
        <p>将第三方模型接口转换为 NewAPI 已支持的官方协议。</p>
        <Input label="用户名" value={form.username} onChange={(v) => setForm({ ...form, username: v })} />
        <Input label="密码" type="password" value={form.password} onChange={(v) => setForm({ ...form, password: v })} />
        <button className="primary full" disabled={submitting}>{submitting ? "登录中..." : "登录"}</button>
        {message && <p className="error">{message}</p>}
      </form>
    </div>
  );
}

function Dashboard({ api, navigate }) {
  const { data, loading, error, reload } = useLoader(async () => {
    const [providers, mappings, logs] = await Promise.all([
      api.get("/api/providers"),
      api.get("/api/mappings"),
      api.get("/api/request-logs?limit=20"),
    ]);
    return { providers: providers.data || [], mappings: mappings.data || [], logs: logs.data || [] };
  }, [api]);

  const providers = data?.providers || [];
  const mappings = data?.mappings || [];
  const logs = data?.logs || [];

  return (
    <Page title="工作台" subtitle="查看渠道接入与转发运行状态。" action={<button onClick={reload}>刷新</button>}>
      <StateBlock loading={loading} error={error}>
        <div className="metric-grid">
          <Metric title="第三方渠道商" value={providers.length} desc={`启用 ${providers.filter((item) => item.enabled).length} 个`} />
          <Metric title="模型映射" value={mappings.length} desc={`启用 ${mappings.filter((item) => item.enabled).length} 条`} />
          <Metric title="最近请求" value={logs.length} desc="最近 20 条调用记录" />
          <Metric title="官方协议" value="OpenAI" desc="图片生成 / 图片编辑" />
        </div>
        <div className="dashboard-grid">
          <Card title="推荐操作流程">
            <div className="setup-list">
              <div><b>1</b><span>进入第三方渠道商，创建一个真实上游渠道。</span></div>
              <div><b>2</b><span>打开渠道详情，在该渠道下新增模型映射。</span></div>
              <div><b>3</b><span>进入映射配置页面，选择官方接口并配置字段映射。</span></div>
              <div><b>4</b><span>NewAPI 中按官方渠道接入本服务。</span></div>
            </div>
            <pre>{`NewAPI 渠道类型：OpenAI
Base URL：http://127.0.0.1:8088
模型名称：渠道详情中配置的公开模型名
API Key：第三方上游 Key，或使用渠道商固定 Key`}</pre>
          </Card>
          <Card title="快捷入口">
            <div className="quick-actions">
              <button onClick={() => navigate("provider-create")}>新增第三方渠道商</button>
              <button className="secondary" onClick={() => navigate("providers")}>查看渠道商列表</button>
              <button className="secondary" onClick={() => navigate("logs")}>查看请求日志</button>
            </div>
          </Card>
        </div>
      </StateBlock>
    </Page>
  );
}

function ProviderListPage({ api, navigate }) {
  const [keyword, setKeyword] = useState("");
  const { data, loading, error, reload } = useLoader(async () => {
    const [providers, mappings] = await Promise.all([api.get("/api/providers"), api.get("/api/mappings")]);
    return { providers: providers.data || [], mappings: mappings.data || [] };
  }, [api]);

  const rows = data?.providers || [];
  const mappings = data?.mappings || [];
  const filtered = rows.filter((row) => `${row.code} ${row.name} ${row.type} ${row.base_url}`.toLowerCase().includes(keyword.toLowerCase()));

  return (
    <Page title="第三方渠道商" subtitle="先创建真实上游渠道，再在渠道详情中配置模型映射。" action={<button onClick={() => navigate("provider-create")}>新增渠道商</button>}>
      <StateBlock loading={loading} error={error}>
        <Card title="渠道商列表" action={<ToolbarSearch value={keyword} onChange={setKeyword} onRefresh={reload} />}>
          <DataTable columns={["渠道名称", "渠道编码", "类型", "Base URL", "映射数量", "状态", "操作"]} rows={filtered.map((row) => [
            row.name || "-",
            row.code,
            row.type || "-",
            <span className="break-cell">{row.base_url}</span>,
            mappings.filter((item) => item.provider_code === row.code).length,
            <Badge tone={row.enabled ? "green" : "gray"}>{row.enabled ? "启用" : "停用"}</Badge>,
            <div className="table-actions">
              <button className="text-button" onClick={() => navigate("provider-detail", { code: row.code })}>详情</button>
              <button className="text-button" onClick={() => navigate("provider-edit", { id: row.id })}>编辑</button>
            </div>,
          ])} empty="暂无渠道商，请点击右上角新增" />
        </Card>
      </StateBlock>
    </Page>
  );
}

function ProviderFormPage({ api, navigate, providerId }) {
  const isEdit = !!providerId;
  const [form, setForm] = useState(PROVIDER_EMPTY);
  const [loading, setLoading] = useState(isEdit);
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (!providerId) return;
    let alive = true;
    setLoading(true);
    api.get("/api/providers").then((res) => {
      const row = (res.data || []).find((item) => item.id === providerId);
      if (alive && row) setForm({ ...PROVIDER_EMPTY, ...row });
    }).catch((err) => alive && setMessage(err.message)).finally(() => alive && setLoading(false));
    return () => { alive = false; };
  }, [api, providerId]);

  async function save() {
    setMessage("");
    const payload = { ...form, timeout_seconds: Number(form.timeout_seconds) || 180 };
    try {
      const res = await api.json(isEdit ? `/api/providers/${providerId}` : "/api/providers", isEdit ? "PUT" : "POST", payload);
      navigate("provider-detail", { code: res.code || payload.code });
    } catch (err) {
      setMessage(err.message);
    }
  }

  return (
    <Page title={isEdit ? "编辑渠道商" : "新增渠道商"} subtitle="填写第三方上游的基础连接信息，保存后进入详情页配置模型映射。">
      <Breadcrumb items={[["第三方渠道商", () => navigate("providers")], [isEdit ? "编辑渠道商" : "新增渠道商"]]} />
      <StateBlock loading={loading} error={message}>
        <Card title="基础信息" action={<div className="button-row"><button className="secondary" onClick={() => navigate("providers")}>取消</button><button onClick={save}>保存渠道商</button></div>}>
          <div className="form-grid">
            <Input label="渠道编码" value={form.code} placeholder="例如 minimax" onChange={(v) => setForm({ ...form, code: v })} />
            <Input label="渠道名称" value={form.name} placeholder="例如 MiniMax 海外" onChange={(v) => setForm({ ...form, name: v })} />
            <Input label="渠道类型" value={form.type} placeholder="例如 minimax / ali / custom" onChange={(v) => setForm({ ...form, type: v })} />
            <Input label="Base URL" value={form.base_url} placeholder="https://api.example.com" onChange={(v) => setForm({ ...form, base_url: v })} />
            <Input label="固定 API Key" value={form.api_key} placeholder="为空时透传 NewAPI 的 Authorization" onChange={(v) => setForm({ ...form, api_key: v })} />
            <Input label="超时时间（秒）" type="number" value={form.timeout_seconds} onChange={(v) => setForm({ ...form, timeout_seconds: v })} />
            <Checkbox label="启用渠道" checked={form.enabled} onChange={(v) => setForm({ ...form, enabled: v })} />
          </div>
        </Card>
      </StateBlock>
    </Page>
  );
}

function ProviderDetailPage({ api, navigate, providerCode }) {
  const { data, loading, error, reload } = useLoader(async () => {
    const [providers, mappings] = await Promise.all([api.get("/api/providers"), api.get("/api/mappings")]);
    const provider = (providers.data || []).find((item) => item.code === providerCode);
    return { provider, mappings: (mappings.data || []).filter((item) => item.provider_code === providerCode) };
  }, [api, providerCode]);

  const provider = data?.provider;
  const mappings = data?.mappings || [];

  return (
    <Page title="渠道商详情" subtitle="在渠道详情中集中管理该渠道下的模型映射。" action={<button onClick={reload}>刷新</button>}>
      <Breadcrumb items={[["第三方渠道商", () => navigate("providers")], [provider?.name || providerCode || "详情"]]} />
      <StateBlock loading={loading} error={error || (!provider ? "未找到该渠道商" : "")}>
        {provider && (
          <>
            <Card title="基础信息" action={<div className="button-row"><button className="secondary" onClick={() => navigate("provider-edit", { id: provider.id })}>编辑基础信息</button><button onClick={() => navigate("mapping-create", { providerCode: provider.code })}>新增模型映射</button></div>}>
              <div className="detail-grid">
                <Info label="渠道名称" value={provider.name} />
                <Info label="渠道编码" value={provider.code} />
                <Info label="渠道类型" value={provider.type} />
                <Info label="Base URL" value={provider.base_url} />
                <Info label="超时时间" value={`${provider.timeout_seconds || 180} 秒`} />
                <Info label="渠道状态" value={provider.enabled ? "启用" : "停用"} />
              </div>
            </Card>
            <Card title="模型映射" subtitle="公开模型名是 NewAPI 对外使用的模型名；上游模型名是真实第三方模型名。">
              <DataTable columns={["映射名称", "公开模型", "官方接口", "上游模型", "上游路径", "状态", "操作"]} rows={mappings.map((row) => [
                row.name,
                row.public_model,
                endpointLabel(row.target_endpoint),
                row.upstream_model,
                <span className="break-cell">{row.upstream_path}</span>,
                <Badge tone={row.enabled ? "green" : "gray"}>{row.enabled ? "启用" : "停用"}</Badge>,
                <button className="text-button" onClick={() => navigate("mapping-edit", { id: row.id, providerCode: provider.code })}>查看 / 编辑</button>,
              ])} empty="该渠道还没有模型映射，请点击右上角新增" />
            </Card>
          </>
        )}
      </StateBlock>
    </Page>
  );
}

function MappingFormPage({ api, navigate, mappingId, providerCode }) {
  const isEdit = !!mappingId;
  const [form, setForm] = useState({ ...MAPPING_EMPTY, provider_code: providerCode || "" });
  const [endpoints, setEndpoints] = useState([]);
  const [providers, setProviders] = useState([]);
  const [paramRows, setParamRows] = useState([]);
  const [responseRows, setResponseRows] = useState([]);
  const [errorRows, setErrorRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState("");
  const selectedEndpoint = endpoints.find((item) => item.key === form.target_endpoint) || endpoints[0];

  useEffect(() => {
    let alive = true;
    async function load() {
      setLoading(true);
      setMessage("");
      try {
        const [endpointRes, providerRes, mappingRes] = await Promise.all([
          api.get("/api/official/endpoints"),
          api.get("/api/providers"),
          isEdit ? api.get("/api/mappings") : Promise.resolve({ data: [] }),
        ]);
        if (!alive) return;
        const endpointList = endpointRes.data || [];
        const nextProviders = providerRes.data || [];
        const current = isEdit ? (mappingRes.data || []).find((item) => item.id === mappingId) : null;
        const nextForm = current ? { ...MAPPING_EMPTY, ...current } : { ...MAPPING_EMPTY, provider_code: providerCode || "" };
        const endpoint = endpointList.find((item) => item.key === nextForm.target_endpoint) || endpointList[0];
        setEndpoints(endpointList);
        setProviders(nextProviders);
        setForm(nextForm);
        if (endpoint) {
          setParamRows(createParamRows(endpoint, nextForm));
          setResponseRows(createResponseRows(endpoint, nextForm));
          setErrorRows(createErrorRows(endpoint, nextForm));
        }
      } catch (err) {
        if (alive) setMessage(err.message);
      } finally {
        if (alive) setLoading(false);
      }
    }
    load();
    return () => { alive = false; };
  }, [api, mappingId, providerCode, isEdit]);

  function changeEndpoint(key) {
    const endpoint = endpoints.find((item) => item.key === key);
    if (!endpoint) return;
    const next = {
      ...form,
      target_protocol: endpoint.protocol,
      target_endpoint: endpoint.key,
      upstream_method: endpoint.method,
      body_mode: endpoint.upstream_body_mode || endpoint.body_mode,
    };
    setForm(next);
    setParamRows(createParamRows(endpoint, next));
    setResponseRows(createResponseRows(endpoint, next));
    setErrorRows(createErrorRows(endpoint, next));
  }

  async function save() {
    setMessage("");
    const mapping = buildMappingJSON(paramRows);
    const responseMapping = buildResponseMappingJSON(responseRows);
    const errorMapping = buildErrorMappingJSON(errorRows);
    const payload = {
      ...form,
      target_protocol: selectedEndpoint?.protocol || form.target_protocol,
      target_endpoint: selectedEndpoint?.key || form.target_endpoint,
      field_map_json: stringifyJSON(mapping.fieldMap),
      file_field_map_json: stringifyJSON(mapping.fileFieldMap),
      defaults_json: stringifyJSON(mapping.defaults),
      ignore_fields_json: stringifyJSON(mapping.ignoreFields),
      header_map_json: form.header_map_json || "{}",
      response_field_map_json: stringifyJSON(responseMapping.fieldMap),
      response_defaults_json: stringifyJSON(responseMapping.defaults),
      error_field_map_json: stringifyJSON(errorMapping.fieldMap),
      error_defaults_json: stringifyJSON(errorMapping.defaults),
    };
    try {
      const res = await api.json(isEdit ? `/api/mappings/${mappingId}` : "/api/mappings", isEdit ? "PUT" : "POST", payload);
      navigate("provider-detail", { code: res.provider_code || payload.provider_code });
    } catch (err) {
      setMessage(err.message);
    }
  }

  return (
    <Page title={isEdit ? "编辑模型映射" : "新增模型映射"} subtitle="在指定渠道商下配置公开模型、上游模型以及字段转换规则。">
      <Breadcrumb items={[["第三方渠道商", () => navigate("providers")], [form.provider_code || providerCode || "渠道详情", () => navigate("provider-detail", { code: form.provider_code || providerCode })], [isEdit ? "编辑模型映射" : "新增模型映射"]]} />
      <StateBlock loading={loading} error={message}>
        <Card title="基础映射信息" action={<div className="button-row"><button className="secondary" onClick={() => navigate("provider-detail", { code: form.provider_code || providerCode })}>取消</button><button onClick={save}>保存映射</button></div>}>
          <div className="form-grid">
            <Select label="第三方渠道商" value={form.provider_code} options={providers.map((item) => ({ label: `${item.name || item.code}（${item.code}）`, value: item.code }))} onChange={(v) => setForm({ ...form, provider_code: v })} />
            <Input label="映射名称" value={form.name} placeholder="例如 Canvas-20 图片生成" onChange={(v) => setForm({ ...form, name: v })} />
            <Select label="官方目标接口" value={form.target_endpoint} options={endpoints.map((item) => ({ label: endpointLabel(item.key), value: item.key }))} onChange={changeEndpoint} />
            <Input label="对外模型名" value={form.public_model} placeholder="NewAPI 使用的模型名" onChange={(v) => setForm({ ...form, public_model: v })} />
            <Input label="上游真实模型" value={form.upstream_model} placeholder="第三方真实模型名" onChange={(v) => setForm({ ...form, upstream_model: v })} />
            <Input label="模型字段名" value={form.upstream_model_field} placeholder="通常是 model" onChange={(v) => setForm({ ...form, upstream_model_field: v })} />
            <Input label="上游 Method" value={form.upstream_method} onChange={(v) => setForm({ ...form, upstream_method: v })} />
            <Input label="上游路径" value={form.upstream_path} placeholder="/v1/xxx" onChange={(v) => setForm({ ...form, upstream_path: v })} />
            <Select label="上游请求体" value={form.body_mode} options={["json", "multipart"]} onChange={(v) => setForm({ ...form, body_mode: v })} />
            <Checkbox label="标准化 OpenAI usage" checked={form.normalize_openai_usage} onChange={(v) => setForm({ ...form, normalize_openai_usage: v })} />
            <Checkbox label="启用映射" checked={form.enabled} onChange={(v) => setForm({ ...form, enabled: v })} />
          </div>
        </Card>
        {selectedEndpoint && (
          <Card title="官方接口说明">
            <EndpointPreview endpoint={selectedEndpoint} />
          </Card>
        )}
        <Card title="字段映射配置" subtitle="映射：复制官方字段到上游字段；默认值：固定发送某字段；忽略：丢弃该字段。">
          <ParameterTable rows={paramRows} onChange={setParamRows} />
          <details className="json-preview">
            <summary>查看生成的映射 JSON</summary>
            <pre>{stringifyJSON(buildMappingJSON(paramRows), 2)}</pre>
          </details>
        </Card>
        <Card title="返参映射配置" subtitle="把第三方上游返回字段映射到官方返回字段。路径统一使用 created、data[].url、usage.prompt_tokens 这种写法。">
          <ResponseMappingTable rows={responseRows} onChange={setResponseRows} />
          <details className="json-preview">
            <summary>查看生成的返参映射 JSON</summary>
            <pre>{stringifyJSON(buildResponseMappingJSON(responseRows), 2)}</pre>
          </details>
        </Card>
        <Card title="错误返参映射配置" subtitle="上游返回非 2xx 时，把错误体映射成 OpenAI 兼容的 error.message、error.type、error.param 和 error.code。">
          <ResponseMappingTable rows={errorRows} onChange={setErrorRows} />
          <details className="json-preview">
            <summary>查看生成的错误映射 JSON</summary>
            <pre>{stringifyJSON(buildErrorMappingJSON(errorRows), 2)}</pre>
          </details>
        </Card>
      </StateBlock>
    </Page>
  );
}

function UsersPage({ api }) {
  const [rows, setRows] = useState([]);
  const [keyword, setKeyword] = useState("");
  const [modal, setModal] = useState(null);
  const [form, setForm] = useState(USER_EMPTY);
  const { data, loading, error, reload } = useLoader(async () => {
    const res = await api.get("/api/users");
    return res.data || [];
  }, [api]);

  useEffect(() => { if (data) setRows(data); }, [data]);

  async function saveUser() {
    await api.json(modal?.id ? `/api/users/${modal.id}` : "/api/users", modal?.id ? "PUT" : "POST", form);
    setModal(null);
    setForm(USER_EMPTY);
    reload();
  }

  const filtered = rows.filter((row) => `${row.username} ${row.role}`.toLowerCase().includes(keyword.toLowerCase()));

  return (
    <Page title="用户管理" subtitle="管理渠道适配网关后台账号。" action={<button onClick={() => { setForm(USER_EMPTY); setModal({ mode: "create" }); }}>新增用户</button>}>
      <StateBlock loading={loading} error={error}>
        <Card title="用户列表" action={<ToolbarSearch value={keyword} onChange={setKeyword} onRefresh={reload} />}>
          <DataTable columns={["ID", "用户名", "角色", "状态", "最后登录", "操作"]} rows={filtered.map((row) => [
            row.id,
            row.username,
            row.role,
            <Badge tone={row.enabled ? "green" : "gray"}>{row.enabled ? "启用" : "停用"}</Badge>,
            row.last_login_at ? new Date(row.last_login_at).toLocaleString() : "-",
            <button className="text-button" onClick={() => { setForm({ username: row.username, password: "", role: row.role, enabled: row.enabled }); setModal({ mode: "edit", id: row.id }); }}>编辑</button>,
          ])} empty="暂无用户" />
        </Card>
      </StateBlock>
      {modal && (
        <Modal title={modal.mode === "edit" ? "编辑用户" : "新增用户"} onClose={() => setModal(null)} footer={<><button className="secondary" onClick={() => setModal(null)}>取消</button><button onClick={saveUser}>保存</button></>}>
          <div className="modal-form">
            <Input label="用户名" value={form.username} onChange={(v) => setForm({ ...form, username: v })} />
            <Input label="密码" type="password" placeholder={modal.mode === "edit" ? "留空则不修改密码" : ""} value={form.password} onChange={(v) => setForm({ ...form, password: v })} />
            <Input label="角色" value={form.role} onChange={(v) => setForm({ ...form, role: v })} />
            <Checkbox label="启用用户" checked={form.enabled} onChange={(v) => setForm({ ...form, enabled: v })} />
          </div>
        </Modal>
      )}
    </Page>
  );
}

function LogsPage({ api }) {
  const { data, loading, error, reload } = useLoader(async () => {
    const res = await api.get("/api/request-logs?limit=100");
    return res.data || [];
  }, [api]);
  const rows = data || [];

  return (
    <Page title="请求日志" subtitle="查看 NewAPI 经由网关转发到第三方渠道的最近请求。" action={<button onClick={reload}>刷新日志</button>}>
      <StateBlock loading={loading} error={error}>
        <Card title="最近 100 条请求">
          <DataTable columns={["时间", "公开模型", "官方接口", "上游模型", "渠道商", "状态码", "耗时", "Trace ID", "错误"]} rows={rows.map((row) => [
            new Date(row.created_at).toLocaleString(),
            row.public_model,
            endpointLabel(row.target_endpoint),
            row.upstream_model,
            row.provider_code,
            <Badge tone={row.status_code >= 200 && row.status_code < 300 ? "green" : "red"}>{row.status_code || "-"}</Badge>,
            `${row.latency_ms || 0} ms`,
            row.trace_id || "-",
            row.error_message || "-",
          ])} empty="暂无请求日志" />
        </Card>
      </StateBlock>
    </Page>
  );
}

function ParameterTable({ rows, onChange }) {
  function update(index, patch) {
    onChange(rows.map((item, i) => (i === index ? { ...item, ...patch } : item)));
  }

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>官方字段</th>
            <th>类型</th>
            <th>必填</th>
            <th>处理方式</th>
            <th>上游字段 / 默认值</th>
            <th>说明</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={row.name}>
              <td><strong>{row.name}</strong><span className="field-location">{row.location}</span></td>
              <td>{row.type}</td>
              <td>{row.required ? "是" : "否"}</td>
              <td>
                <select value={row.action} onChange={(e) => update(index, { action: e.target.value })}>
                  <option value="map">映射</option>
                  <option value="default">默认值</option>
                  <option value="ignore">忽略</option>
                </select>
              </td>
              <td>
                {row.action === "default" ? (
                  <div className="inline-fields">
                    <input value={row.upstream_field} placeholder="上游字段" onChange={(e) => update(index, { upstream_field: e.target.value })} />
                    <input value={row.default_value} placeholder="默认值" onChange={(e) => update(index, { default_value: e.target.value })} />
                  </div>
                ) : (
                  <input disabled={row.action === "ignore"} value={row.upstream_field} onChange={(e) => update(index, { upstream_field: e.target.value })} />
                )}
              </td>
              <td>{row.description}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ResponseMappingTable({ rows, onChange }) {
  function update(index, patch) {
    onChange(rows.map((item, i) => (i === index ? { ...item, ...patch } : item)));
  }

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>官方返回字段</th>
            <th>类型</th>
            <th>处理方式</th>
            <th>上游返回字段 / 默认值</th>
            <th>说明</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={row.name}>
              <td><strong>{row.name}</strong><span className="field-location">json</span></td>
              <td>{row.type}</td>
              <td>
                <select value={row.action} onChange={(e) => update(index, { action: e.target.value })}>
                  <option value="map">映射</option>
                  <option value="default">默认值</option>
                  <option value="ignore">不处理</option>
                </select>
              </td>
              <td>
                {row.action === "default" ? (
                  <input value={row.default_value} placeholder="默认值" onChange={(e) => update(index, { default_value: e.target.value })} />
                ) : (
                  <input disabled={row.action === "ignore"} value={row.upstream_field} placeholder="例如 images[].url" onChange={(e) => update(index, { upstream_field: e.target.value })} />
                )}
              </td>
              <td>{row.description}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function EndpointPreview({ endpoint }) {
  return (
    <div className="endpoint-preview">
      <div>
        <strong>官方入参</strong>
        <ul>{endpoint.params.map((param) => <li key={param.name}>{param.name}<span>{param.type}</span></li>)}</ul>
      </div>
      <div>
        <strong>标准返参</strong>
        <ul>{endpoint.response_fields.map((field) => <li key={field.name}>{field.name}<span>{field.type}</span></li>)}</ul>
      </div>
      <div>
        <strong>错误返参</strong>
        <ul>{(endpoint.error_fields || []).map((field) => <li key={field.name}>{field.name}<span>{field.type}</span></li>)}</ul>
      </div>
    </div>
  );
}

function Page({ title, subtitle, action, children }) {
  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h1>{title}</h1>
          <p>{subtitle}</p>
        </div>
        {action}
      </header>
      {children}
    </section>
  );
}

function Card({ title, subtitle, action, children }) {
  return (
    <section className="card">
      <div className="card-header">
        <div>
          <h2>{title}</h2>
          {subtitle && <p>{subtitle}</p>}
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}

function Modal({ title, children, footer, onClose }) {
  return (
    <div className="modal-mask">
      <div className="modal">
        <div className="modal-header">
          <h2>{title}</h2>
          <button className="icon-button" onClick={onClose}>×</button>
        </div>
        <div className="modal-body">{children}</div>
        <div className="modal-footer">{footer}</div>
      </div>
    </div>
  );
}

function Breadcrumb({ items }) {
  return (
    <div className="breadcrumb">
      {items.map((item, index) => {
        const [label, action] = Array.isArray(item) ? item : [item, null];
        return <React.Fragment key={`${label}-${index}`}>{index > 0 && <span>/</span>}{action ? <button onClick={action}>{label}</button> : <strong>{label}</strong>}</React.Fragment>;
      })}
    </div>
  );
}

function Metric({ title, value, desc }) {
  return <div className="metric-card"><span>{title}</span><strong>{value}</strong><small>{desc}</small></div>;
}

function Info({ label, value }) {
  return <div className="info-cell"><span>{label}</span><strong>{value || "-"}</strong></div>;
}

function ToolbarSearch({ value, onChange, onRefresh }) {
  return <div className="toolbar"><input className="search" placeholder="搜索" value={value} onChange={(e) => onChange(e.target.value)} /><button className="secondary" onClick={onRefresh}>刷新</button></div>;
}

function Badge({ tone = "gray", children }) {
  return <span className={`badge ${tone}`}>{children}</span>;
}

function StateBlock({ loading, error, children }) {
  if (loading) return <div className="state-card">加载中...</div>;
  if (error) return <div className="state-card error-box">{error}</div>;
  return children;
}

function Input({ label, type = "text", value, onChange, placeholder = "" }) {
  return <label>{label}<input type={type} value={value ?? ""} placeholder={placeholder} onChange={(e) => onChange(e.target.value)} /></label>;
}

function Select({ label, value, options, onChange }) {
  const normalized = options.map((item) => typeof item === "string" ? { label: item, value: item } : item);
  return (
    <label>{label}
      <select value={value ?? ""} onChange={(e) => onChange(e.target.value)}>
        <option value="">请选择</option>
        {normalized.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
      </select>
    </label>
  );
}

function Checkbox({ label, checked, onChange }) {
  return <label className="checkbox"><input type="checkbox" checked={!!checked} onChange={(e) => onChange(e.target.checked)} />{label}</label>;
}

function DataTable({ columns, rows, empty = "暂无数据" }) {
  return (
    <div className="table-wrap">
      <table>
        <thead><tr>{columns.map((column) => <th key={column}>{column}</th>)}</tr></thead>
        <tbody>
          {rows.length === 0 && <tr><td className="table-empty" colSpan={columns.length}>{empty}</td></tr>}
          {rows.map((row, i) => <tr key={i}>{row.map((cell, j) => <td key={j}>{cell}</td>)}</tr>)}
        </tbody>
      </table>
    </div>
  );
}

function useLoader(loader, deps) {
  const [state, setState] = useState({ data: null, loading: true, error: "" });
  const reload = async () => {
    setState((prev) => ({ ...prev, loading: true, error: "" }));
    try {
      const data = await loader();
      setState({ data, loading: false, error: "" });
    } catch (err) {
      setState({ data: null, loading: false, error: err.message });
    }
  };
  useEffect(() => { reload(); }, deps);
  return { ...state, reload };
}

function createParamRows(endpoint, mapping) {
  const fieldMap = safeJSON(mapping.field_map_json, {});
  const fileFieldMap = safeJSON(mapping.file_field_map_json, {});
  const defaults = safeJSON(mapping.defaults_json, {});
  const ignoreFields = safeJSON(mapping.ignore_fields_json, []);
  const ignored = new Set(Array.isArray(ignoreFields) ? ignoreFields : []);

  return (endpoint?.params || []).map((param) => {
    const mappedField = param.location === "file" ? fileFieldMap[param.name] : fieldMap[param.name];
    const defaultKey = Object.prototype.hasOwnProperty.call(defaults, param.name)
      ? param.name
      : mappedField && Object.prototype.hasOwnProperty.call(defaults, mappedField)
        ? mappedField
        : "";
    const hasDefault = defaultKey !== "";
    const action = ignored.has(param.name) ? "ignore" : hasDefault ? "default" : "map";
    return {
      ...param,
      action,
      upstream_field: mappedField || param.name,
      default_value: hasDefault ? stringifyValue(defaults[defaultKey]) : stringifyValue(param.default ?? ""),
    };
  });
}

function createResponseRows(endpoint, mapping) {
  const fieldMap = safeJSON(mapping.response_field_map_json, {});
  const defaults = safeJSON(mapping.response_defaults_json, {});

  return (endpoint?.response_fields || []).map((field) => {
    const mappedField = fieldMap[field.name] || field.name;
    const hasDefault = Object.prototype.hasOwnProperty.call(defaults, field.name);
    return {
      ...field,
      action: hasDefault ? "default" : "map",
      upstream_field: mappedField,
      default_value: hasDefault ? stringifyValue(defaults[field.name]) : stringifyValue(field.default ?? ""),
    };
  });
}

function createErrorRows(endpoint, mapping) {
  const fieldMap = safeJSON(mapping.error_field_map_json, {});
  const defaults = safeJSON(mapping.error_defaults_json, {});

  return (endpoint?.error_fields || []).map((field) => {
    const mappedField = fieldMap[field.name] || defaultErrorUpstreamField(field.name);
    const hasDefault = Object.prototype.hasOwnProperty.call(defaults, field.name);
    return {
      ...field,
      action: hasDefault ? "default" : "map",
      upstream_field: mappedField,
      default_value: hasDefault ? stringifyValue(defaults[field.name]) : stringifyValue(field.default ?? ""),
    };
  });
}

function buildMappingJSON(paramRows) {
  const fieldMap = {};
  const fileFieldMap = {};
  const defaults = {};
  const ignoreFields = [];

  paramRows.forEach((row) => {
    if (row.action === "ignore") {
      ignoreFields.push(row.name);
      return;
    }
    if (row.action === "default") {
      defaults[row.upstream_field || row.name] = parseDefaultValue(row.default_value);
      return;
    }
    if (!row.upstream_field || row.upstream_field === row.name) return;
    if (row.location === "file") fileFieldMap[row.name] = row.upstream_field;
    else fieldMap[row.name] = row.upstream_field;
  });

  return { fieldMap, fileFieldMap, defaults, ignoreFields };
}

function buildResponseMappingJSON(responseRows) {
  const fieldMap = {};
  const defaults = {};

  responseRows.forEach((row) => {
    if (row.action === "ignore") return;
    if (row.action === "default") {
      defaults[row.name] = parseDefaultValue(row.default_value);
      return;
    }
    if (row.upstream_field) {
      fieldMap[row.name] = row.upstream_field;
    }
  });

  return { fieldMap, defaults };
}

function buildErrorMappingJSON(errorRows) {
  return buildResponseMappingJSON(errorRows);
}

function defaultErrorUpstreamField(name) {
  const defaults = {
    "error.message": "base_resp.status_msg",
    "error.type": "type",
    "error.param": "param",
    "error.code": "base_resp.status_code",
  };
  return defaults[name] || name;
}

function safeJSON(raw, fallback) {
  if (!raw) return fallback;
  try {
    return JSON.parse(raw);
  } catch {
    return fallback;
  }
}

function stringifyJSON(value, space = 0) {
  return JSON.stringify(value ?? {}, null, space);
}

function stringifyValue(value) {
  if (value === undefined || value === null) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function parseDefaultValue(value) {
  const text = String(value ?? "").trim();
  if (text === "") return "";
  if (text === "true") return true;
  if (text === "false") return false;
  if (text === "null") return null;
  if (/^-?\d+(\.\d+)?$/.test(text)) return Number(text);
  if ((text.startsWith("{") && text.endsWith("}")) || (text.startsWith("[") && text.endsWith("]"))) {
    try {
      return JSON.parse(text);
    } catch {
      return text;
    }
  }
  return text;
}

function endpointLabel(key) {
  const labels = {
    "openai.images.generations": "OpenAI 图片生成",
    "openai.images.edits": "OpenAI 图片编辑",
  };
  return labels[key] || key || "-";
}

createRoot(document.getElementById("root")).render(<App />);
