import { useI18n } from '@/i18n';
import {
  createWebDataSession,
  deleteWebDataCredential,
  deleteWebDataSession,
  executeWebDataStatement,
  getWebDataAudits,
  getWebDataMetadata,
  getWebDataObject,
  getWebDataTarget,
  saveWebDataCredential,
  testWebDataConnection,
} from '@/services/api';
import {
  ApiOutlined,
  AuditOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloudServerOutlined,
  ClusterOutlined,
  CopyOutlined,
  DatabaseOutlined,
  DeleteOutlined,
  DisconnectOutlined,
  DownloadOutlined,
  EditOutlined,
  FileSearchOutlined,
  FileTextOutlined,
  KeyOutlined,
  LoginOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  SaveOutlined,
  SearchOutlined,
  SettingOutlined,
  TableOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import { history, useModel, useParams } from '@umijs/max';
import {
  Alert,
  Button,
  Collapse,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Tree,
  Typography,
  message,
} from 'antd';
import type { ChangeEvent, KeyboardEvent, UIEvent } from 'react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { renderWebDataAuditDetails } from './auditDetails';
import {
  buildCompletionContext,
  buildCompletionItems,
  completionTypeIcon,
  getStatementTextArea,
  getTextareaCompletionPosition,
  renderStatementHighlight,
  shouldChainCompletionAfterApply,
  shouldOpenCompletionForContext,
  statementPlaceholder,
} from './completion';
import {
  buildConnectionProfileFromValues,
  buildConnectionRequestFromValues,
  connectionMeta,
  connectionSubtitle,
  connectionTitle,
  defaultConnectionName,
  normalizeConnectionTLSMode,
  tlsOptionsForProtocol,
} from './connection';
import './index.less';
import {
  buildGenericColumns,
  buildObjectParams,
  buildResultColumns,
  editableCellValue,
  filterMetadataNodes,
  formatAuditTime,
  formatCellValue,
  isInspectableNode,
  mapMetadataTree,
  summarizeMetadata,
  summarizeResult,
  unionRowKeys,
} from './metadata';
import {
  buildMongoDeleteCommand,
  buildMongoInsertCommand,
  buildMongoUpdateCommand,
  buildMongoUpdateFieldCommand,
  buildObjectFilterCommand,
  buildObjectTemplate,
  buildRedisMemberCommand,
  buildRedisRowDeleteCommand,
  buildRedisSetCommand,
  buildSQLDeleteRowCommand,
  buildSQLInsertCommand,
  buildSQLUpdateCellCommand,
  buildSQLUpdateRowCommand,
  canAddRedisMember,
  canDeleteRedisRow,
  canIdentifyMongoRow,
  canIdentifySQLRow,
  canUpdateRedisRow,
  coerceFilterOperator,
  createDefaultObjectFilter,
  createObjectFilterCondition,
  defaultRedisMemberValues,
  filterOperatorNeedsValue,
  hasRedisRowActions,
  mongoDocumentIdentity,
  mongoEditableDocument,
  normalizeObjectFilter,
  objectFilterFieldInfos,
  objectFilterFieldKind,
  objectFilterOperatorOptions,
  objectFilterValuePlaceholder,
  quoteRedisArg,
  redisActionDescription,
  redisObjectType,
  sqlEditableColumns,
  sqlIdentityColumn,
} from './objectCommands';
import { isSQLProtocol, protocolLabels } from './protocol';
import {
  currentUserStorageKey,
  objectStatementDraftKey,
  readStatementDraft,
  writeStatementDraft,
} from './statementDraft';
import {
  buildExplainStatement,
  downloadTextFile,
  isDangerousStatement,
  rowsToCSV,
} from './statementUtils';
import type {
  CompletionItem,
  CompletionPosition,
  ExecuteGeneratedCommandOptions,
  ExecuteStatementOptions,
  InlineCellEdit,
  LoadObjectDetailOptions,
  MetadataTreeNode,
  ObjectFilterCondition,
  ObjectFilterFieldInfo,
  ObjectFilterState,
  QuickAction,
  SQLRowAction,
} from './types';

const { Text, Title } = Typography;
const { TextArea } = Input;

const WebDataPage: React.FC = () => {
  const { tr } = useI18n();
  const { initialState } = useModel('@@initialState');
  const params = useParams();
  const proxyId = Number(params.proxyId);
  const currentUserKey = useMemo(
    () => currentUserStorageKey(initialState?.currentUser),
    [
      initialState?.currentUser?.email,
      initialState?.currentUser?.id,
      initialState?.currentUser?.name,
    ],
  );
  const [connectionForm] = Form.useForm();
  const [quickForm] = Form.useForm();
  const statementTextAreaRef = useRef<any>(null);
  const [loading, setLoading] = useState(true);
  const [connecting, setConnecting] = useState(false);
  const [testingConnection, setTestingConnection] = useState(false);
  const [connectionSaving, setConnectionSaving] = useState(false);
  const [connectingCredentialId, setConnectingCredentialId] = useState<
    number | undefined
  >();
  const [connectionDrawerOpen, setConnectionDrawerOpen] = useState(false);
  const [editingCredential, setEditingCredential] =
    useState<API.WebDataCredential>();
  const [executing, setExecuting] = useState(false);
  const [metadataLoading, setMetadataLoading] = useState(false);
  const [objectLoading, setObjectLoading] = useState(false);
  const [auditLoading, setAuditLoading] = useState(false);
  const [auditOpen, setAuditOpen] = useState(false);
  const [target, setTarget] = useState<API.WebDataTarget>();
  const [session, setSession] = useState<API.CreateWebDataSessionResponse>();
  const [statement, setStatement] = useState('');
  const [result, setResult] = useState<API.WebDataExecuteResult>();
  const [metadata, setMetadata] = useState<API.WebDataMetadataNode[]>([]);
  const [selectedNode, setSelectedNode] = useState<API.WebDataMetadataNode>();
  const [objectDetail, setObjectDetail] = useState<API.WebDataObjectResult>();
  const [auditItems, setAuditItems] = useState<API.WebDataAuditItem[]>([]);
  const [quickAction, setQuickAction] = useState<QuickAction>();
  const [sqlRowAction, setSQLRowAction] = useState<SQLRowAction>();
  const [sqlActionRow, setSQLActionRow] = useState<Record<string, any>>();
  const [mongoActionRow, setMongoActionRow] = useState<Record<string, any>>();
  const [objectFilter, setObjectFilter] = useState<ObjectFilterState>({
    conditions: [],
    limit: 100,
  });
  const [objectDetailOpen, setObjectDetailOpen] = useState(false);
  const objectRequestSeq = useRef(0);
  const skipStatementDraftSaveRef = useRef(false);
  const [treeSearch, setTreeSearch] = useState('');
  const [editorCursor, setEditorCursor] = useState(0);
  const [editorScrollTop, setEditorScrollTop] = useState(0);
  const [completionActive, setCompletionActive] = useState(false);
  const [completionForced, setCompletionForced] = useState(false);
  const [completionIndex, setCompletionIndex] = useState(0);
  const [completionPosition, setCompletionPosition] =
    useState<CompletionPosition>({
      left: 12,
      top: 40,
      width: 420,
      maxHeight: 288,
    });
  const [inlineCellEdit, setInlineCellEdit] = useState<InlineCellEdit>();
  const [inlineCellSaving, setInlineCellSaving] = useState(false);
  const connected = Boolean(session?.token);

  useEffect(() => {
    let mounted = true;
    const loadTarget = async () => {
      setLoading(true);
      try {
        const res = await getWebDataTarget(proxyId);
        if (!mounted) return;
        if (res.code === 200 && res.data) {
          setTarget(res.data);
          setStatement('');
          connectionForm.setFieldsValue({
            protocol: res.data.protocol,
            redis_db: 0,
            tls_mode: 'disable',
            direct_connection: true,
          });
        } else {
          message.error(
            res.message || tr('加载目标失败', 'Failed to load target'),
          );
        }
      } catch (err: any) {
        if (mounted)
          message.error(
            err?.message || tr('加载目标失败', 'Failed to load target'),
          );
      } finally {
        if (mounted) setLoading(false);
      }
    };
    loadTarget();
    return () => {
      mounted = false;
    };
  }, [connectionForm, proxyId, tr]);

  useEffect(() => {
    return () => {
      if (session?.token) {
        deleteWebDataSession(session.token).catch(() => {});
      }
    };
  }, [session?.token]);

  const filteredMetadata = useMemo(
    () => filterMetadataNodes(metadata, treeSearch),
    [metadata, treeSearch],
  );

  const treeData = useMemo<MetadataTreeNode[]>(
    () => mapMetadataTree(filteredMetadata),
    [filteredMetadata],
  );

  const metadataSummary = useMemo(
    () => summarizeMetadata(metadata),
    [metadata],
  );

  const resultSummary = useMemo(() => summarizeResult(result), [result]);

  const statementDraftKey = useMemo(
    () =>
      objectStatementDraftKey(
        currentUserKey,
        proxyId,
        target?.protocol,
        objectDetail,
      ),
    [
      currentUserKey,
      objectDetail?.database,
      objectDetail?.key,
      objectDetail?.name,
      objectDetail?.object_type,
      objectDetail?.schema,
      proxyId,
      target?.protocol,
    ],
  );

  useEffect(() => {
    if (!statementDraftKey || !isSQLProtocol(target?.protocol)) return;
    if (skipStatementDraftSaveRef.current) {
      skipStatementDraftSaveRef.current = false;
      return;
    }
    const timer = window.setTimeout(() => {
      writeStatementDraft(statementDraftKey, statement);
    }, 400);
    return () => window.clearTimeout(timer);
  }, [statement, statementDraftKey, target?.protocol]);

  const completionContext = useMemo(
    () => buildCompletionContext(statement, editorCursor),
    [editorCursor, statement],
  );

  const completionItems = useMemo(
    () =>
      buildCompletionItems(
        target?.protocol,
        metadata,
        selectedNode,
        completionContext,
        tr,
      ),
    [completionContext, metadata, selectedNode, target?.protocol, tr],
  );

  const completionContextOpen = shouldOpenCompletionForContext(
    target?.protocol,
    completionContext,
  );

  const completionVisible =
    connected &&
    completionActive &&
    completionItems.length > 0 &&
    (completionForced ||
      completionContext.query.length > 0 ||
      completionContextOpen);
  const editorLineNumbers = useMemo(
    () => statement.split('\n').map((_, index) => index + 1),
    [statement],
  );
  const editorActiveLine = useMemo(
    () => statement.slice(0, editorCursor).split('\n').length,
    [editorCursor, statement],
  );

  useEffect(() => {
    if (completionIndex >= completionItems.length) {
      setCompletionIndex(0);
    }
  }, [completionIndex, completionItems.length]);

  useEffect(() => {
    setCompletionIndex(0);
  }, [
    completionContext.previousWord,
    completionContext.query,
    selectedNode?.key,
    target?.protocol,
  ]);

  useEffect(() => {
    if (!completionVisible || typeof window === 'undefined') return;
    const textarea = getStatementTextArea(statementTextAreaRef.current);
    if (!textarea) return;
    const updatePosition = () => {
      setCompletionPosition(
        getTextareaCompletionPosition(textarea, editorCursor),
      );
    };
    const frame = window.requestAnimationFrame(updatePosition);
    textarea.addEventListener('scroll', updatePosition);
    window.addEventListener('resize', updatePosition);
    return () => {
      window.cancelAnimationFrame(frame);
      textarea.removeEventListener('scroll', updatePosition);
      window.removeEventListener('resize', updatePosition);
    };
  }, [completionVisible, editorCursor, statement]);

  useEffect(() => {
    const fields = objectFilterFieldInfos(target?.protocol, objectDetail);
    setObjectFilter((prev) => normalizeObjectFilter(prev, fields));
  }, [objectDetail, target?.protocol]);

  const reloadTarget = async () => {
    const res = await getWebDataTarget(proxyId);
    if (res.code === 200 && res.data) {
      setTarget(res.data);
      return res.data;
    }
    message.error(res.message || tr('加载目标失败', 'Failed to load target'));
    return undefined;
  };

  const activateSession = async (data: API.CreateWebDataSessionResponse) => {
    objectRequestSeq.current += 1;
    setSession(data);
    setResult(undefined);
    setSelectedNode(undefined);
    setObjectDetail(undefined);
    setConnectionDrawerOpen(false);
    message.success(tr('已连接', 'Connected'));
    await loadMetadata(data.token);
  };

  const handleConnect = async (values: any) => {
    if (!target) return false;
    setConnecting(true);
    try {
      const res = await createWebDataSession(
        proxyId,
        buildConnectionRequestFromValues(target.protocol, values, {
          saveCredential: values.save_credential,
          credential: editingCredential,
        }),
      );
      if (res.code !== 200 || !res.data) {
        message.error(res.message || tr('连接失败', 'Connection failed'));
        return false;
      }
      await activateSession(res.data);
      return true;
    } catch (err: any) {
      message.error(err?.message || tr('连接失败', 'Connection failed'));
      return false;
    } finally {
      setConnecting(false);
    }
  };

  const handleConnectWithCredential = async (
    credential: API.WebDataCredential,
  ) => {
    if (!target) return false;
    setConnecting(true);
    setConnectingCredentialId(credential.id);
    try {
      const res = await createWebDataSession(proxyId, {
        credential_id: credential.id,
        protocol: target.protocol,
        username: credential.username,
        database: credential.database,
        auth_database: credential.auth_database,
        redis_db: credential.redis_db,
        tls_mode: credential.tls_mode,
        schema: credential.schema,
        auth_mechanism: credential.auth_mechanism,
        direct_connection: credential.direct_connection,
        connection_params: credential.connection_params,
      });
      if (res.code !== 200 || !res.data) {
        message.error(res.message || tr('连接失败', 'Connection failed'));
        return false;
      }
      await activateSession(res.data);
      return true;
    } catch (err: any) {
      message.error(err?.message || tr('连接失败', 'Connection failed'));
      return false;
    } finally {
      setConnecting(false);
      setConnectingCredentialId(undefined);
    }
  };

  const handleDisconnect = async () => {
    if (!session?.token) return;
    try {
      await deleteWebDataSession(session.token);
    } catch {}
    setSession(undefined);
    setMetadata([]);
    objectRequestSeq.current += 1;
    setSelectedNode(undefined);
    setObjectDetail(undefined);
    setResult(undefined);
    message.success(tr('已断开', 'Disconnected'));
  };

  const openCreateConnection = () => {
    if (!target) return;
    setEditingCredential(undefined);
    connectionForm.resetFields();
    connectionForm.setFieldsValue({
      protocol: target.protocol,
      name: defaultConnectionName(target.protocol),
      redis_db: 0,
      tls_mode: 'disable',
      direct_connection: true,
    });
    setConnectionDrawerOpen(true);
  };

  const openEditConnection = (credential: API.WebDataCredential) => {
    if (!target) return;
    setEditingCredential(credential);
    connectionForm.resetFields();
    connectionForm.setFieldsValue({
      id: credential.id,
      protocol: credential.protocol || target.protocol,
      name: credential.name || connectionTitle(credential),
      username: credential.username,
      password: '',
      database: credential.database,
      auth_database: credential.auth_database,
      redis_db: credential.redis_db ?? 0,
      tls_mode: normalizeConnectionTLSMode(credential.tls_mode),
      schema: credential.schema,
      auth_mechanism: credential.auth_mechanism,
      direct_connection: credential.direct_connection ?? true,
      connection_params: credential.connection_params,
    });
    setConnectionDrawerOpen(true);
  };

  const closeConnectionDrawer = () => {
    setConnectionDrawerOpen(false);
    setEditingCredential(undefined);
    connectionForm.resetFields();
  };

  const handleSaveConnection = async (connectAfterSave = false) => {
    if (!target) return;
    let values: any;
    try {
      values = await connectionForm.validateFields();
    } catch {
      return;
    }
    setConnectionSaving(true);
    try {
      const res = await saveWebDataCredential(proxyId, {
        id: editingCredential?.id,
        name: values.name?.trim(),
        password: values.password,
        ...buildConnectionProfileFromValues(target.protocol, values, {
          credential: editingCredential,
        }),
      });
      if (res.code !== 200 || !res.data) {
        message.error(res.message || tr('保存失败', 'Save failed'));
        return;
      }
      message.success(tr('连接已保存', 'Connection saved'));
      closeConnectionDrawer();
      await reloadTarget();
      if (connectAfterSave) {
        await handleConnectWithCredential(res.data);
      }
    } catch (err: any) {
      message.error(err?.message || tr('保存失败', 'Save failed'));
    } finally {
      setConnectionSaving(false);
    }
  };

  const handleConnectOnceFromDrawer = async () => {
    if (!target) return;
    let values: any;
    try {
      values = await connectionForm.validateFields();
    } catch {
      return;
    }
    if (editingCredential?.id && !values.password) {
      setConnecting(true);
      try {
        const res = await createWebDataSession(
          proxyId,
          buildConnectionRequestFromValues(target.protocol, values, {
            credentialId: editingCredential.id,
            credential: editingCredential,
          }),
        );
        if (res.code !== 200 || !res.data) {
          message.error(res.message || tr('连接失败', 'Connection failed'));
          return;
        }
        await activateSession(res.data);
      } catch (err: any) {
        message.error(err?.message || tr('连接失败', 'Connection failed'));
      } finally {
        setConnecting(false);
      }
      return;
    }
    await handleConnect({
      ...values,
      protocol: target.protocol,
      save_credential: false,
    });
  };

  const handleTestConnectionFromDrawer = async () => {
    if (!target) return;
    let values: any;
    try {
      values = await connectionForm.validateFields();
    } catch {
      return;
    }
    setTestingConnection(true);
    try {
      const useSavedCredential = Boolean(
        editingCredential?.id && !values.password,
      );
      const res = await testWebDataConnection(
        proxyId,
        buildConnectionRequestFromValues(target.protocol, values, {
          credentialId: useSavedCredential ? editingCredential?.id : undefined,
          credential: editingCredential,
        }),
      );
      if (res.code === 200) {
        message.success(
          res.message || tr('连接测试成功', 'Connection test succeeded'),
        );
        return;
      }
      message.error(tr('测试连接失败', 'Connection test failed'));
    } catch {
      message.error(tr('测试连接失败', 'Connection test failed'));
    } finally {
      setTestingConnection(false);
    }
  };

  const loadMetadata = async (token = session?.token) => {
    if (!token) return;
    setMetadataLoading(true);
    try {
      const res = await getWebDataMetadata(token);
      if (res.code === 200 && res.data) {
        setMetadata(res.data.nodes || []);
      }
    } catch (err: any) {
      message.error(
        err?.message || tr('加载元数据失败', 'Failed to load metadata'),
      );
    } finally {
      setMetadataLoading(false);
    }
  };

  const loadAudits = async () => {
    if (!proxyId) return;
    setAuditLoading(true);
    try {
      const res = await getWebDataAudits(proxyId, { limit: 100 });
      if (res.code === 200 && res.data) {
        setAuditItems(res.data.items || []);
      } else {
        message.error(
          res.message || tr('加载审计失败', 'Failed to load audits'),
        );
      }
    } catch (err: any) {
      message.error(
        err?.message || tr('加载审计失败', 'Failed to load audits'),
      );
    } finally {
      setAuditLoading(false);
    }
  };

  const openAuditDrawer = async () => {
    setAuditOpen(true);
    await loadAudits();
  };

  const loadObjectDetail = async (
    node: API.WebDataMetadataNode,
    options: LoadObjectDetailOptions = {},
  ) => {
    if (!session?.token || !target) return;
    if (statementDraftKey && isSQLProtocol(target.protocol)) {
      writeStatementDraft(statementDraftKey, statement);
    }
    const updateStatement = options.updateStatement ?? true;
    const updateResult = options.updateResult ?? true;
    const clearResult = options.clearResult ?? updateResult;
    const objectParams = buildObjectParams(target.protocol, node);
    if (!objectParams) {
      objectRequestSeq.current += 1;
      setObjectLoading(false);
      setObjectDetail(undefined);
      if (clearResult) {
        setResult(undefined);
      }
      return;
    }
    const requestSeq = objectRequestSeq.current + 1;
    objectRequestSeq.current = requestSeq;
    setObjectLoading(true);
    setSelectedNode(node);
    setObjectDetail(undefined);
    setInlineCellEdit(undefined);
    if (clearResult) {
      setResult(undefined);
    }
    try {
      const res = await getWebDataObject(session.token, objectParams);
      if (requestSeq !== objectRequestSeq.current) {
        return;
      }
      if (res.code === 200 && res.data) {
        const detail = res.data;
        setObjectDetail(detail);
        const previewStatement = buildObjectTemplate(
          target.protocol,
          detail,
          'preview',
        );
        const draftKey = objectStatementDraftKey(
          currentUserKey,
          proxyId,
          target.protocol,
          detail,
        );
        const savedStatement = readStatementDraft(draftKey);
        const nextStatement = savedStatement || previewStatement;
        if (nextStatement && updateStatement) {
          skipStatementDraftSaveRef.current = true;
          setStatement(nextStatement);
          setEditorCursor(nextStatement.length);
        }
      } else {
        message.error(
          res.message || tr('加载对象失败', 'Failed to load object'),
        );
      }
    } catch (err: any) {
      if (requestSeq !== objectRequestSeq.current) {
        return;
      }
      message.error(
        err?.message || tr('加载对象失败', 'Failed to load object'),
      );
    } finally {
      if (requestSeq === objectRequestSeq.current) {
        setObjectLoading(false);
      }
    }
  };

  const getRunnableStatement = () => {
    const textarea = getStatementTextArea(statementTextAreaRef.current);
    const selectionStart = textarea?.selectionStart ?? 0;
    const selectionEnd = textarea?.selectionEnd ?? 0;
    if (textarea && selectionEnd > selectionStart) {
      return {
        command: textarea.value.slice(selectionStart, selectionEnd).trim(),
        fromSelection: true,
      };
    }
    return {
      command: statement.trim(),
      fromSelection: false,
    };
  };

  const runStatement = async () => {
    if (!session?.token) return;
    if (statementDraftKey && isSQLProtocol(target?.protocol)) {
      writeStatementDraft(statementDraftKey, statement);
    }
    const runnable = getRunnableStatement();
    if (!runnable.command) {
      message.warning(
        runnable.fromSelection
          ? tr('选中的命令为空', 'Selected command is empty')
          : tr('请输入命令', 'Enter a command'),
      );
      return;
    }
    const execute = async () => executeStatement(runnable.command);

    if (isDangerousStatement(target?.protocol, runnable.command)) {
      Modal.confirm({
        title: tr('确认执行危险操作？', 'Run dangerous operation?'),
        content: tr(
          '该命令可能修改或删除数据。系统会记录审计，但不会阻止执行。',
          'This command may modify or delete data. It will be audited, but not blocked.',
        ),
        okText: tr('确认执行', 'Run'),
        okButtonProps: { danger: true },
        cancelText: tr('取消', 'Cancel'),
        onOk: execute,
      });
      return;
    }
    await execute();
  };

  const executeStatement = async (
    command: string,
    options: ExecuteStatementOptions = {},
  ) => {
    if (!session?.token) return false;
    const refreshMetadata = options.refreshMetadata ?? true;
    setInlineCellEdit(undefined);
    setExecuting(true);
    try {
      const res = await executeWebDataStatement(session.token, command);
      if (res.code === 200 && res.data) {
        setResult(res.data);
        if (res.data.error) {
          message.error(res.data.error);
          return false;
        } else {
          message.success(res.data.message || tr('执行完成', 'Executed'));
          if (refreshMetadata) {
            loadMetadata().catch(() => {});
            if (selectedNode) {
              loadObjectDetail(selectedNode, {
                clearResult: false,
                updateResult: false,
                updateStatement: false,
              }).catch(() => {});
            }
          }
        }
        if (auditOpen) {
          loadAudits().catch(() => {});
        }
        return true;
      } else {
        message.error(res.message || tr('执行失败', 'Execution failed'));
        return false;
      }
    } catch (err: any) {
      message.error(err?.message || tr('执行失败', 'Execution failed'));
      return false;
    } finally {
      setExecuting(false);
    }
  };

  const updateEditorCursor = () => {
    const textarea = getStatementTextArea(statementTextAreaRef.current);
    if (textarea) {
      setEditorCursor(textarea.selectionStart ?? statement.length);
      setEditorScrollTop(textarea.scrollTop);
    }
  };

  const replaceStatement = (next: string) => {
    setStatement(next);
    setEditorCursor(next.length);
  };

  const applyCompletion = (item: CompletionItem) => {
    const textarea = getStatementTextArea(statementTextAreaRef.current);
    const cursor = textarea?.selectionStart ?? editorCursor;
    const context = buildCompletionContext(statement, cursor);
    const next =
      statement.slice(0, context.start) +
      item.insertText +
      statement.slice(context.end);
    const nextCursor = context.start + item.insertText.length;
    const shouldChain = shouldChainCompletionAfterApply(
      target?.protocol,
      next,
      nextCursor,
    );
    replaceStatement(next);
    setEditorCursor(nextCursor);
    setCompletionActive(true);
    setCompletionForced(shouldChain);
    setCompletionIndex(0);
    window.setTimeout(() => {
      const nextTextarea = getStatementTextArea(statementTextAreaRef.current);
      nextTextarea?.focus();
      nextTextarea?.setSelectionRange(nextCursor, nextCursor);
      setEditorCursor(nextCursor);
      setCompletionActive(true);
      setCompletionForced(shouldChain);
    }, 0);
  };

  const handleStatementChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    replaceStatement(event.target.value);
    setEditorCursor(event.target.selectionStart ?? event.target.value.length);
    setEditorScrollTop(event.target.scrollTop);
    setCompletionActive(true);
    setCompletionForced(false);
  };

  const handleStatementScroll = (event: UIEvent<HTMLTextAreaElement>) => {
    setEditorScrollTop(event.currentTarget.scrollTop);
  };

  const handleStatementKeyDown = (
    event: KeyboardEvent<HTMLTextAreaElement>,
  ) => {
    if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
      event.preventDefault();
      runStatement();
      return;
    }
    if ((event.metaKey || event.ctrlKey) && event.key === ' ') {
      event.preventDefault();
      updateEditorCursor();
      setCompletionActive(true);
      setCompletionForced(true);
      return;
    }
    if (!completionVisible) return;
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setCompletionIndex((prev) => (prev + 1) % completionItems.length);
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setCompletionIndex(
        (prev) => (prev - 1 + completionItems.length) % completionItems.length,
      );
      return;
    }
    if (event.key === 'Tab') {
      event.preventDefault();
      applyCompletion(completionItems[completionIndex] || completionItems[0]);
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      applyCompletion(completionItems[completionIndex] || completionItems[0]);
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      setCompletionActive(false);
      setCompletionForced(false);
    }
  };

  const handleExplainStatement = async () => {
    if (!isSQLProtocol(target?.protocol)) {
      message.info(tr('Explain 仅支持 SQL 类型', 'Explain only supports SQL'));
      return;
    }
    const trimmed = statement.trim();
    if (!trimmed) {
      message.warning(tr('请输入命令', 'Enter a command'));
      return;
    }
    await executeStatement(buildExplainStatement(trimmed), {
      refreshMetadata: false,
    });
  };

  const handleExportResult = (format: 'csv' | 'json') => {
    if (!result?.rows?.length) {
      message.warning(tr('没有可导出的结果', 'No rows to export'));
      return;
    }
    const filename = `webdata-result-${new Date()
      .toISOString()
      .replace(/[:.]/g, '-')}.${format}`;
    if (format === 'json') {
      downloadTextFile(
        filename,
        JSON.stringify(result.rows, null, 2),
        'application/json;charset=utf-8',
      );
      return;
    }
    const columns = result.columns?.length
      ? result.columns
      : unionRowKeys(result.rows || []);
    downloadTextFile(
      filename,
      rowsToCSV(result.rows, columns),
      'text/csv;charset=utf-8',
    );
  };

  const handleDeleteCredential = async (credential: API.WebDataCredential) => {
    if (!target) return;
    Modal.confirm({
      title: tr('删除保存的连接？', 'Delete saved connection?'),
      content: connectionTitle(credential),
      okText: tr('删除', 'Delete'),
      okButtonProps: { danger: true },
      cancelText: tr('取消', 'Cancel'),
      onOk: async () => {
        try {
          await deleteWebDataCredential(proxyId, {
            id: credential.id,
            protocol: credential.protocol,
            username: credential.username,
            database: credential.database,
            auth_database: credential.auth_database,
          });
          message.success(tr('已删除连接', 'Connection deleted'));
          await reloadTarget();
        } catch (err: any) {
          message.error(err?.message || tr('删除失败', 'Delete failed'));
        }
      },
    });
  };

  const handleTreeSelect = (_keys: React.Key[], info: any) => {
    const source = (info.node as MetadataTreeNode).source;
    if (!source) return;
    setSelectedNode(source);
    if (isInspectableNode(source)) {
      loadObjectDetail(source).catch(() => {});
      return;
    }
    objectRequestSeq.current += 1;
    setObjectLoading(false);
    setObjectDetail(undefined);
    setResult(undefined);
  };

  const applyObjectTemplate = (action: string) => {
    if (!target || !objectDetail) return;
    const next = buildObjectTemplate(target.protocol, objectDetail, action);
    if (!next) {
      message.info(tr('当前对象暂无可生成模板', 'No template for this object'));
      return;
    }
    replaceStatement(next);
    message.success(tr('已生成命令', 'Command generated'));
  };

  const refreshCurrentObjectPreview = async () => {
    if (!target || !objectDetail) return;
    const command = buildObjectTemplate(
      target.protocol,
      objectDetail,
      'preview',
    );
    if (!command) return;
    await executeStatement(command, { refreshMetadata: false });
  };

  const executeGeneratedCommand = async (
    command: string,
    options: ExecuteGeneratedCommandOptions = {},
  ) => {
    if (!command.trim()) return false;
    if (options.updateStatement !== false) {
      replaceStatement(command);
    }
    const execute = async () => {
      const ok = await executeStatement(command, {
        refreshMetadata: options.refreshMetadata,
      });
      if (ok) {
        await options.afterSuccess?.();
      }
      return ok;
    };
    if (isDangerousStatement(target?.protocol, command)) {
      Modal.confirm({
        title: tr('确认执行写入操作？', 'Run write operation?'),
        content: tr(
          '该操作会修改目标数据。系统会记录审计，但不会阻止执行。',
          'This operation will modify target data. It will be audited, but not blocked.',
        ),
        okText: tr('确认执行', 'Run'),
        okButtonProps: { danger: true },
        cancelText: tr('取消', 'Cancel'),
        onOk: execute,
      });
      return false;
    }
    return execute();
  };

  const buildCurrentObjectFilterCommand = () => {
    if (!target || !objectDetail) return;
    const fieldInfos = objectFilterFieldInfos(target.protocol, objectDetail);
    if (!fieldInfos.length) {
      message.warning(tr('当前对象暂无可筛选字段', 'No filterable fields'));
      return;
    }
    try {
      const normalizedFilter = normalizeObjectFilter(objectFilter, fieldInfos);
      setObjectFilter(normalizedFilter);
      const command = buildObjectFilterCommand(
        target.protocol,
        objectDetail,
        normalizedFilter,
      );
      return command;
    } catch (err: any) {
      message.error(
        err?.message || tr('生成筛选失败', 'Failed to build filter'),
      );
    }
  };

  const applyObjectFilter = async () => {
    const command = buildCurrentObjectFilterCommand();
    if (!command) return;
    await executeGeneratedCommand(command, {
      refreshMetadata: false,
      updateStatement: false,
    });
  };

  const resetObjectFilter = async () => {
    if (!target || !objectDetail) return;
    const fieldInfos = objectFilterFieldInfos(target.protocol, objectDetail);
    setObjectFilter(createDefaultObjectFilter(fieldInfos[0]?.name));
    const command = buildObjectTemplate(
      target.protocol,
      objectDetail,
      'preview',
    );
    if (command) {
      await executeGeneratedCommand(command, {
        refreshMetadata: false,
        updateStatement: false,
      });
    }
  };

  const openQuickAction = (action: Exclude<QuickAction, undefined>) => {
    if (!target || !objectDetail) return;
    if (action === 'sql-insert') {
      const columns = sqlEditableColumns(objectDetail);
      quickForm.setFieldsValue({
        values: Object.fromEntries(columns.map((column) => [column.name, ''])),
      });
    }
    if (action === 'redis-set') {
      quickForm.setFieldsValue({
        key: objectDetail.key || objectDetail.name || '',
        value: '',
        ttl: undefined,
      });
    }
    if (action === 'redis-member-add') {
      quickForm.setFieldsValue(defaultRedisMemberValues(objectDetail, 'add'));
    }
    if (action === 'mongo-insert') {
      quickForm.setFieldsValue({
        document: JSON.stringify({ name: 'example' }, null, 2),
      });
    }
    setQuickAction(action);
  };

  const closeQuickAction = () => {
    setQuickAction(undefined);
    setSQLRowAction(undefined);
    setSQLActionRow(undefined);
    setMongoActionRow(undefined);
    quickForm.resetFields();
  };

  const openSQLRowUpdate = (row: Record<string, any>) => {
    if (!target || !objectDetail || !isSQLProtocol(target.protocol)) return;
    const identity = sqlIdentityColumn(target.protocol, objectDetail, row);
    const columns = sqlEditableColumns(objectDetail).filter(
      (column) => column.name !== identity.column,
    );
    quickForm.setFieldsValue({
      where_column: identity.column,
      where_value: identity.value,
      values: Object.fromEntries(
        columns.map((column) => [column.name, row[column.name] ?? '']),
      ),
    });
    setSQLActionRow(row);
    setSQLRowAction('sql-update');
  };

  const deleteSQLRow = (row: Record<string, any>) => {
    if (!target || !objectDetail || !isSQLProtocol(target.protocol)) return;
    let command = '';
    try {
      command = buildSQLDeleteRowCommand(target.protocol, objectDetail, row);
    } catch (err: any) {
      message.error(
        err?.message || tr('生成命令失败', 'Failed to build command'),
      );
      return;
    }
    executeGeneratedCommand(command, {
      afterSuccess: refreshCurrentObjectPreview,
      updateStatement: false,
    }).catch(() => {});
  };

  const openMongoDocumentUpdate = (row: Record<string, any>) => {
    if (!target || !objectDetail || target.protocol !== 'mongodb') return;
    try {
      const identity = mongoDocumentIdentity(row);
      quickForm.setFieldsValue({
        filter: JSON.stringify({ _id: identity }, null, 2),
        document: JSON.stringify(mongoEditableDocument(row), null, 2),
      });
      setMongoActionRow(row);
      setQuickAction('mongo-update');
    } catch (err: any) {
      message.error(
        err?.message || tr('生成命令失败', 'Failed to build command'),
      );
    }
  };

  const deleteMongoDocument = (row: Record<string, any>) => {
    if (!target || !objectDetail || target.protocol !== 'mongodb') return;
    let command = '';
    try {
      command = buildMongoDeleteCommand(objectDetail, row);
    } catch (err: any) {
      message.error(
        err?.message || tr('生成命令失败', 'Failed to build command'),
      );
      return;
    }
    executeGeneratedCommand(command, {
      afterSuccess: refreshCurrentObjectPreview,
      updateStatement: false,
    }).catch(() => {});
  };

  const deleteRedisKey = () => {
    if (!target || !objectDetail || target.protocol !== 'redis') return;
    const key = objectDetail.key || objectDetail.name || '';
    if (!key) {
      message.error(tr('Key 不能为空', 'Key is required'));
      return;
    }
    executeGeneratedCommand(`DEL ${quoteRedisArg(key)}`, {
      afterSuccess: refreshCurrentObjectPreview,
      updateStatement: false,
    }).catch(() => {});
  };

  const openRedisRowUpdate = (row: Record<string, any>) => {
    if (!target || !objectDetail || target.protocol !== 'redis') return;
    try {
      quickForm.setFieldsValue(
        defaultRedisMemberValues(objectDetail, 'update', row),
      );
      setQuickAction('redis-member-update');
    } catch (err: any) {
      message.error(
        err?.message || tr('生成命令失败', 'Failed to build command'),
      );
    }
  };

  const deleteRedisRow = (row: Record<string, any>) => {
    if (!target || !objectDetail || target.protocol !== 'redis') return;
    let command = '';
    try {
      command = buildRedisRowDeleteCommand(objectDetail, row);
    } catch (err: any) {
      message.error(
        err?.message || tr('生成命令失败', 'Failed to build command'),
      );
      return;
    }
    executeGeneratedCommand(command, {
      afterSuccess: refreshCurrentObjectPreview,
      updateStatement: false,
    }).catch(() => {});
  };

  const handleQuickActionSubmit = async (values: any) => {
    if (!target || !objectDetail || (!quickAction && !sqlRowAction)) return;
    let command = '';
    try {
      if (quickAction === 'sql-insert') {
        command = buildSQLInsertCommand(target.protocol, objectDetail, values);
      }
      if (quickAction === 'redis-set') {
        command = buildRedisSetCommand(values);
      }
      if (
        quickAction === 'redis-member-add' ||
        quickAction === 'redis-member-update'
      ) {
        command = buildRedisMemberCommand(
          objectDetail,
          values,
          quickAction === 'redis-member-update',
        );
      }
      if (quickAction === 'mongo-insert') {
        command = buildMongoInsertCommand(objectDetail, values);
      }
      if (quickAction === 'mongo-update') {
        command = buildMongoUpdateCommand(objectDetail, mongoActionRow, values);
      }
      if (sqlRowAction === 'sql-update') {
        command = buildSQLUpdateRowCommand(
          target.protocol,
          objectDetail,
          values,
          sqlActionRow,
        );
      }
    } catch (err: any) {
      message.error(
        err?.message || tr('生成命令失败', 'Failed to build command'),
      );
      return;
    }
    closeQuickAction();
    await executeGeneratedCommand(command, {
      afterSuccess: refreshCurrentObjectPreview,
      updateStatement: false,
    });
  };

  const copyCurrentStatement = async () => {
    try {
      await navigator.clipboard.writeText(statement);
      message.success(tr('已复制', 'Copied'));
    } catch {
      message.error(tr('复制失败', 'Copy failed'));
    }
  };

  const currentObjectRowActionOptions = () => {
    if (!target || !objectDetail) return {};
    const protocol = target.protocol || '';
    const isSQLTable =
      isSQLProtocol(protocol) && objectDetail.object_type === 'table';
    const isRedisKey =
      protocol === 'redis' && objectDetail.object_type === 'key';
    const isMongoCollection =
      protocol === 'mongodb' && objectDetail.object_type === 'collection';
    return {
      sqlRowActions: isSQLTable,
      redisRowActions: isRedisKey && hasRedisRowActions(objectDetail),
      mongoRowActions: isMongoCollection,
    };
  };

  const inlineCellMode = (
    column: string,
    row: Record<string, any>,
    options: { sqlRowActions?: boolean; mongoRowActions?: boolean },
  ): 'sql' | 'mongo' | undefined => {
    if (!target || !objectDetail || column === '_webdata_key') return undefined;
    if (options.sqlRowActions && isSQLProtocol(target.protocol)) {
      try {
        const identity = sqlIdentityColumn(target.protocol, objectDetail, row);
        const editable = sqlEditableColumns(objectDetail).some(
          (item) => item.name === column,
        );
        return editable && column !== identity.column ? 'sql' : undefined;
      } catch {
        return undefined;
      }
    }
    if (options.mongoRowActions) {
      return canIdentifyMongoRow(row) && column !== '_id' ? 'mongo' : undefined;
    }
    return undefined;
  };

  const startInlineCellEdit = (
    scope: string,
    row: Record<string, any>,
    column: string,
    mode: 'sql' | 'mongo',
  ) => {
    setInlineCellEdit({
      scope,
      rowKey: row._webdata_key,
      column,
      mode,
      row,
      value: editableCellValue(row[column]),
    });
  };

  const saveInlineCellEdit = async () => {
    if (!inlineCellEdit || !target || !objectDetail) return;
    let command = '';
    try {
      if (inlineCellEdit.mode === 'sql') {
        command = buildSQLUpdateCellCommand(
          target.protocol,
          objectDetail,
          inlineCellEdit.row,
          inlineCellEdit.column,
          inlineCellEdit.value,
        );
      } else {
        command = buildMongoUpdateFieldCommand(
          objectDetail,
          inlineCellEdit.row,
          inlineCellEdit.column,
          inlineCellEdit.value,
        );
      }
    } catch (err: any) {
      message.error(
        err?.message ||
          tr('生成更新命令失败', 'Failed to build update command'),
      );
      return;
    }
    setInlineCellSaving(true);
    await executeGeneratedCommand(command, {
      afterSuccess: async () => {
        setInlineCellEdit(undefined);
        await refreshCurrentObjectPreview();
      },
      updateStatement: false,
    });
    setInlineCellSaving(false);
  };

  const renderEditableCell = (
    scope: string,
    column: string,
    value: any,
    row: Record<string, any>,
    options: { sqlRowActions?: boolean; mongoRowActions?: boolean },
  ) => {
    const mode = inlineCellMode(column, row, options);
    const editing =
      inlineCellEdit?.scope === scope &&
      inlineCellEdit.rowKey === row._webdata_key &&
      inlineCellEdit.column === column;
    if (editing) {
      return (
        <Input
          autoFocus
          size="small"
          className="webdata-cell-input"
          value={inlineCellEdit.value}
          disabled={inlineCellSaving}
          onChange={(event) =>
            setInlineCellEdit((prev) =>
              prev ? { ...prev, value: event.target.value } : prev,
            )
          }
          onPressEnter={(event) => {
            event.preventDefault();
            saveInlineCellEdit().catch(() => {});
          }}
          onKeyDown={(event) => {
            if (event.key === 'Escape') {
              event.preventDefault();
              setInlineCellEdit(undefined);
            }
          }}
        />
      );
    }
    const cell = (
      <span
        className={`webdata-cell ${mode ? 'is-editable' : ''}`}
        onDoubleClick={() => {
          if (mode) startInlineCellEdit(scope, row, column, mode);
        }}
      >
        {formatCellValue(value)}
      </span>
    );
    return mode ? (
      <Tooltip
        title={tr('双击编辑，回车保存，Esc 取消', 'Double-click to edit')}
      >
        {cell}
      </Tooltip>
    ) : (
      <Tooltip title={formatCellValue(value)}>{cell}</Tooltip>
    );
  };

  const renderRowsTable = (
    data?: API.WebDataExecuteResult,
    emptyText = tr('暂无结果', 'No result'),
    options: {
      sqlRowActions?: boolean;
      redisRowActions?: boolean;
      mongoRowActions?: boolean;
      tableId?: string;
    } = {},
  ) => {
    if (data?.error) {
      return (
        <Alert
          type="error"
          showIcon
          message={data.error}
          className="webdata-alert"
        />
      );
    }
    if (!data?.rows?.length) {
      return <div className="webdata-empty">{data?.message || emptyText}</div>;
    }
    const tableScope = options.tableId || 'rows';
    const columns: any[] = buildResultColumns(data, (value, column, row) =>
      renderEditableCell(tableScope, column, value, row, options),
    );
    if (
      options.sqlRowActions ||
      options.redisRowActions ||
      options.mongoRowActions
    ) {
      columns.push({
        title: tr('操作', 'Actions'),
        dataIndex: '_webdata_actions',
        key: '_webdata_actions',
        ellipsis: false,
        fixed: 'right',
        width: 150,
        render: (_value: any, row: Record<string, any>) => {
          const actions: React.ReactNode[] = [];
          if (options.redisRowActions && objectDetail) {
            if (canUpdateRedisRow(objectDetail)) {
              actions.push(
                <Button
                  key="update"
                  icon={<EditOutlined />}
                  size="small"
                  onClick={() => openRedisRowUpdate(row)}
                >
                  {tr('编辑', 'Edit')}
                </Button>,
              );
            }
            if (canDeleteRedisRow(objectDetail)) {
              actions.push(
                <Button
                  key="delete"
                  danger
                  icon={<DeleteOutlined />}
                  size="small"
                  onClick={() => deleteRedisRow(row)}
                >
                  {tr('删除', 'Delete')}
                </Button>,
              );
            }
          } else {
            const canIdentify = options.mongoRowActions
              ? canIdentifyMongoRow(row)
              : Boolean(
                  objectDetail &&
                    canIdentifySQLRow(
                      target?.protocol || '',
                      objectDetail,
                      row,
                    ),
                );
            const disabledTitle = options.mongoRowActions
              ? tr('当前文档没有 _id', 'This document has no _id')
              : tr(
                  '无法确定用于更新或删除的字段',
                  'Cannot determine the field for update or delete',
                );
            actions.push(
              <Tooltip key="delete" title={canIdentify ? '' : disabledTitle}>
                <Button
                  danger
                  icon={<DeleteOutlined />}
                  size="small"
                  disabled={!canIdentify}
                  onClick={() =>
                    options.mongoRowActions
                      ? deleteMongoDocument(row)
                      : deleteSQLRow(row)
                  }
                >
                  {tr('删除', 'Delete')}
                </Button>
              </Tooltip>,
            );
          }
          return <Space size={4}>{actions}</Space>;
        },
      });
    }
    const rows = data.rows.map((row, index) => ({
      ...row,
      _webdata_key: index,
    }));
    return (
      <Table
        className="webdata-table"
        size="small"
        rowKey="_webdata_key"
        columns={columns}
        dataSource={rows}
        pagination={{ pageSize: 50, showSizeChanger: true }}
        scroll={{ x: true, y: 420 }}
      />
    );
  };

  const renderMapTable = (rows?: Record<string, any>[], emptyText?: string) => {
    if (!rows?.length) {
      return (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={emptyText} />
      );
    }
    return (
      <Table
        className="webdata-table"
        size="small"
        rowKey={(_, index) => String(index)}
        columns={buildGenericColumns(rows)}
        dataSource={rows}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        scroll={{ x: true, y: 360 }}
      />
    );
  };

  const renderObjectFilter = (fieldInfos: ObjectFilterFieldInfo[]) => {
    if (!fieldInfos.length) return null;
    const fields = fieldInfos.map((field) => field.name);
    const fieldOptions = fieldInfos.map((field) => ({
      label: field.typeLabel
        ? `${field.name} · ${field.typeLabel}`
        : field.name,
      value: field.name,
    }));
    const conditions = objectFilter.conditions.length
      ? objectFilter.conditions
      : [createObjectFilterCondition(fields[0])];
    const updateCondition = (
      id: string,
      patch: Partial<ObjectFilterCondition>,
    ) => {
      setObjectFilter((prev) => ({
        ...prev,
        conditions: (prev.conditions.length ? prev.conditions : conditions).map(
          (condition) =>
            condition.id === id ? { ...condition, ...patch } : condition,
        ),
      }));
    };
    const updateConditionField = (
      condition: ObjectFilterCondition,
      field: string,
    ) => {
      const kind = objectFilterFieldKind(fieldInfos, field);
      const operator = coerceFilterOperator(condition.operator, kind);
      updateCondition(condition.id, {
        field,
        operator,
        value: filterOperatorNeedsValue(operator) ? condition.value : '',
      });
    };
    const addCondition = () => {
      setObjectFilter((prev) => ({
        ...prev,
        conditions: [
          ...(prev.conditions.length ? prev.conditions : conditions),
          createObjectFilterCondition(fields[0]),
        ],
      }));
    };
    const removeCondition = (id: string) => {
      setObjectFilter((prev) => {
        const nextConditions = (
          prev.conditions.length ? prev.conditions : conditions
        ).filter((condition) => condition.id !== id);
        return {
          ...prev,
          conditions: nextConditions.length
            ? nextConditions
            : [createObjectFilterCondition(fields[0])],
        };
      });
    };
    return (
      <div className="webdata-filter-panel">
        <div className="webdata-filter-head">
          <Space size={6}>
            <SearchOutlined />
            <Text strong>{tr('快捷筛选', 'Quick Filter')}</Text>
            <Text type="secondary">{tr('多条件按 AND 组合', 'AND logic')}</Text>
          </Space>
          <Space size={6}>
            <Text type="secondary">Limit</Text>
            <InputNumber
              size="small"
              className="webdata-filter-limit"
              min={1}
              max={1000}
              precision={0}
              value={objectFilter.limit}
              onChange={(limit) =>
                setObjectFilter((prev) => ({
                  ...prev,
                  limit: Number(limit || 100),
                }))
              }
            />
            <Button size="small" icon={<PlusOutlined />} onClick={addCondition}>
              {tr('条件', 'Condition')}
            </Button>
          </Space>
        </div>
        <div className="webdata-filter-rows">
          {conditions.map((condition, index) => {
            const field = condition.field || fields[0];
            const fieldKind = objectFilterFieldKind(fieldInfos, field);
            const operator = coerceFilterOperator(
              condition.operator || 'eq',
              fieldKind,
            );
            const needsValue = filterOperatorNeedsValue(operator);
            return (
              <div className="webdata-filter-row" key={condition.id}>
                <span className="webdata-filter-join">
                  {index === 0 ? tr('当', 'Where') : 'AND'}
                </span>
                <Select
                  size="small"
                  showSearch
                  className="webdata-filter-field"
                  value={field}
                  options={fieldOptions}
                  optionFilterProp="label"
                  popupMatchSelectWidth={false}
                  onChange={(nextField) =>
                    updateConditionField(condition, nextField)
                  }
                />
                <Select
                  size="small"
                  className="webdata-filter-operator"
                  value={operator}
                  options={objectFilterOperatorOptions(tr, fieldKind)}
                  onChange={(nextOperator) =>
                    updateCondition(condition.id, {
                      operator: nextOperator,
                      value: filterOperatorNeedsValue(nextOperator)
                        ? condition.value
                        : '',
                    })
                  }
                />
                <Input
                  size="small"
                  className="webdata-filter-value"
                  disabled={!needsValue}
                  value={needsValue ? condition.value : ''}
                  placeholder={
                    needsValue
                      ? objectFilterValuePlaceholder(fieldKind, tr)
                      : '-'
                  }
                  onChange={(event) =>
                    updateCondition(condition.id, {
                      value: event.target.value,
                    })
                  }
                  onPressEnter={() => applyObjectFilter()}
                />
                <Button
                  size="small"
                  icon={<DeleteOutlined />}
                  disabled={conditions.length <= 1}
                  onClick={() => removeCondition(condition.id)}
                />
              </div>
            );
          })}
        </div>
        <div className="webdata-filter-actions">
          <Button
            size="small"
            type="primary"
            icon={<PlayCircleOutlined />}
            loading={executing}
            onClick={() => applyObjectFilter()}
          >
            {tr('执行筛选', 'Run Filter')}
          </Button>
          <Button size="small" onClick={resetObjectFilter}>
            {tr('重置', 'Reset')}
          </Button>
        </div>
      </div>
    );
  };

  const renderConnectionManager = () => {
    const credentials = target?.credentials || [];

    return (
      <div className="webdata-connection-manager">
        <div className="webdata-connection-toolbar">
          <div>
            <Title level={5} className="webdata-section-title">
              {tr('数据库连接', 'Database Connections')}
            </Title>
            <Text type="secondary">
              {credentials.length
                ? tr(
                    '选择已保存连接可直接进入控制台，也可以新建或编辑连接。',
                    'Choose a saved connection to enter the console, or create and edit connections.',
                  )
                : tr(
                    '还没有保存的连接，先新建一个连接配置。',
                    'No saved connection yet. Create one first.',
                  )}
            </Text>
          </div>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={openCreateConnection}
          >
            {tr('新建连接', 'New Connection')}
          </Button>
        </div>

        {credentials.length > 0 ? (
          <div className="webdata-connection-list">
            {credentials.map((credential) => (
              <div
                className="webdata-connection-card"
                key={
                  credential.id ||
                  `${credential.protocol}-${credential.username}-${credential.database}-${credential.auth_database}`
                }
              >
                <div className="webdata-connection-card-main">
                  <div className="webdata-connection-identity">
                    <DatabaseOutlined className="webdata-connection-icon" />
                    <div className="webdata-connection-copy">
                      <div className="webdata-connection-title-row">
                        <Text strong className="webdata-connection-title">
                          {connectionTitle(credential)}
                        </Text>
                        <Tag color="blue">
                          {protocolLabels[credential.protocol] ||
                            credential.protocol}
                        </Tag>
                      </div>
                      <div>
                        <Text
                          type="secondary"
                          className="webdata-connection-subtitle"
                        >
                          {connectionSubtitle(credential)}
                        </Text>
                      </div>
                      <div className="webdata-connection-meta-row">
                        {connectionMeta(credential).map((item) => (
                          <span className="webdata-connection-chip" key={item}>
                            {item}
                          </span>
                        ))}
                        <span className="webdata-connection-chip">
                          <ClockCircleOutlined />
                          {credential.last_used_at
                            ? formatAuditTime(credential.last_used_at)
                            : tr('未使用', 'Never used')}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
                <div className="webdata-connection-actions">
                  <Button
                    type="primary"
                    icon={<LoginOutlined />}
                    loading={
                      connecting && connectingCredentialId === credential.id
                    }
                    disabled={target?.effective_status !== 'active'}
                    onClick={() => handleConnectWithCredential(credential)}
                  >
                    {tr('进入', 'Enter')}
                  </Button>
                  <Button
                    icon={<EditOutlined />}
                    onClick={() => openEditConnection(credential)}
                  >
                    {tr('编辑', 'Edit')}
                  </Button>
                  <Button
                    danger
                    icon={<DeleteOutlined />}
                    onClick={() => handleDeleteCredential(credential)}
                  >
                    {tr('删除', 'Delete')}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="webdata-connection-empty">
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={tr('暂无保存连接', 'No saved connections')}
            >
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={openCreateConnection}
              >
                {tr('新建连接', 'New Connection')}
              </Button>
            </Empty>
          </div>
        )}
      </div>
    );
  };

  const renderConnectionDrawer = () => {
    const protocol = target?.protocol || 'mysql';
    const isSQL = isSQLProtocol(protocol);
    const isRedis = protocol === 'redis';
    const isMongo = protocol === 'mongodb';
    return (
      <Drawer
        title={
          editingCredential
            ? tr('编辑连接', 'Edit Connection')
            : tr('新建连接', 'New Connection')
        }
        width={520}
        open={connectionDrawerOpen}
        onClose={closeConnectionDrawer}
        destroyOnClose
        footer={
          <div className="webdata-drawer-footer">
            <Button onClick={closeConnectionDrawer}>
              {tr('取消', 'Cancel')}
            </Button>
            <Button
              icon={<CheckCircleOutlined />}
              loading={testingConnection}
              disabled={target?.effective_status !== 'active'}
              onClick={handleTestConnectionFromDrawer}
            >
              {tr('测试连接', 'Test')}
            </Button>
            <Button
              icon={<LoginOutlined />}
              loading={connecting}
              disabled={target?.effective_status !== 'active'}
              onClick={handleConnectOnceFromDrawer}
            >
              {tr('连接一次', 'Connect Once')}
            </Button>
            <Button
              icon={<SaveOutlined />}
              loading={connectionSaving}
              onClick={() => handleSaveConnection(false)}
            >
              {tr('保存', 'Save')}
            </Button>
            <Button
              type="primary"
              icon={<LoginOutlined />}
              loading={connectionSaving || connecting}
              disabled={target?.effective_status !== 'active'}
              onClick={() => handleSaveConnection(true)}
            >
              {tr('保存并进入', 'Save and Enter')}
            </Button>
          </div>
        }
      >
        <Form form={connectionForm} layout="vertical">
          <Form.Item name="name" label={tr('连接名称', 'Connection Name')}>
            <Input
              prefix={<SettingOutlined />}
              placeholder={defaultConnectionName(protocol)}
            />
          </Form.Item>
          <Form.Item name="username" label={tr('用户名', 'Username')}>
            <Input prefix={<KeyOutlined />} autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" label={tr('密码', 'Password')}>
            <Input.Password
              autoComplete="current-password"
              placeholder={
                editingCredential
                  ? tr('留空则保留原密码', 'Leave blank to keep password')
                  : undefined
              }
            />
          </Form.Item>
          {!isRedis && (
            <Form.Item
              name="database"
              label={
                isMongo
                  ? tr('默认数据库', 'Default DB')
                  : tr('数据库', 'Database')
              }
            >
              <Input placeholder={isMongo ? 'admin' : undefined} />
            </Form.Item>
          )}
          {isRedis && (
            <Form.Item name="redis_db" label={tr('Redis DB', 'Redis DB')}>
              <InputNumber
                min={0}
                max={15}
                precision={0}
                className="webdata-full"
              />
            </Form.Item>
          )}
          <Collapse
            ghost
            className="webdata-advanced-collapse"
            items={[
              {
                key: 'advanced',
                label: tr('高级配置', 'Advanced Configuration'),
                children: (
                  <>
                    <Form.Item name="tls_mode" label="TLS">
                      <Select options={tlsOptionsForProtocol(protocol, tr)} />
                    </Form.Item>
                    {protocol === 'postgresql' && (
                      <Form.Item
                        name="schema"
                        label={tr('默认 Schema', 'Default Schema')}
                      >
                        <Input placeholder="public" />
                      </Form.Item>
                    )}
                    {isMongo && (
                      <>
                        <Form.Item
                          name="auth_database"
                          label={tr('认证库', 'Auth Database')}
                        >
                          <Input placeholder="admin" />
                        </Form.Item>
                        <Form.Item
                          name="auth_mechanism"
                          label={tr('认证机制', 'Auth Mechanism')}
                        >
                          <Select
                            allowClear
                            options={[
                              { label: 'Default', value: '' },
                              {
                                label: 'SCRAM-SHA-256',
                                value: 'SCRAM-SHA-256',
                              },
                              { label: 'SCRAM-SHA-1', value: 'SCRAM-SHA-1' },
                              { label: 'MONGODB-CR', value: 'MONGODB-CR' },
                              { label: 'PLAIN', value: 'PLAIN' },
                            ]}
                          />
                        </Form.Item>
                        <Form.Item
                          name="direct_connection"
                          label="directConnection"
                          valuePropName="checked"
                        >
                          <Switch />
                        </Form.Item>
                      </>
                    )}
                    {(isSQL || isMongo) && (
                      <Form.Item
                        name="connection_params"
                        label={tr('连接参数', 'Connection Parameters')}
                        extra={tr(
                          '每行一个 key=value，也支持使用 & 分隔。',
                          'One key=value per line. Ampersand-separated values are also supported.',
                        )}
                      >
                        <TextArea
                          autoSize={{ minRows: 3, maxRows: 8 }}
                          className="webdata-textarea"
                          placeholder={
                            protocol === 'mysql'
                              ? 'charset=utf8mb4\nloc=Local'
                              : protocol === 'postgresql'
                              ? 'application_name=liaison-webdata'
                              : 'replicaSet=rs0\nreadPreference=primary'
                          }
                        />
                      </Form.Item>
                    )}
                  </>
                ),
              },
            ]}
          />
          {editingCredential && (
            <Alert
              type="info"
              showIcon
              message={tr(
                '编辑时密码留空会继续使用已保存的加密密码。',
                'Leave password blank while editing to keep the encrypted saved password.',
              )}
            />
          )}
        </Form>
      </Drawer>
    );
  };

  const renderAuditDrawer = () => (
    <Drawer
      title={tr('操作审计', 'Operation Audit')}
      width={820}
      open={auditOpen}
      onClose={() => setAuditOpen(false)}
      extra={
        <Button
          icon={<ReloadOutlined />}
          loading={auditLoading}
          onClick={loadAudits}
        >
          {tr('刷新', 'Refresh')}
        </Button>
      }
    >
      <Table
        size="small"
        rowKey="id"
        loading={auditLoading}
        dataSource={auditItems}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        scroll={{ x: 920 }}
        columns={[
          {
            title: tr('时间', 'Time'),
            dataIndex: 'created_at',
            key: 'created_at',
            width: 170,
            render: (value: string) => formatAuditTime(value),
          },
          {
            title: tr('状态', 'Status'),
            dataIndex: 'success',
            key: 'success',
            width: 90,
            render: (success: boolean) =>
              success ? (
                <Tag color="success">OK</Tag>
              ) : (
                <Tag color="error">ERROR</Tag>
              ),
          },
          {
            title: tr('命令', 'Command'),
            dataIndex: 'statement_preview',
            key: 'statement_preview',
            ellipsis: true,
            render: (value: string) => (
              <Tooltip title={value}>
                <span className="webdata-audit-preview">{value}</span>
              </Tooltip>
            ),
          },
          {
            title: tr('耗时', 'Elapsed'),
            dataIndex: 'elapsed_ms',
            key: 'elapsed_ms',
            width: 90,
            render: (value: number) => `${value || 0} ms`,
          },
        ]}
        expandable={{
          expandedRowRender: (record) => (
            <Space direction="vertical" size={4}>
              <Text type="secondary">SHA256: {record.statement_sha256}</Text>
              {renderWebDataAuditDetails(record.details)}
              {record.error && <Text type="danger">{record.error}</Text>}
            </Space>
          ),
          rowExpandable: (record) =>
            Boolean(record.error || record.statement_sha256),
        }}
      />
    </Drawer>
  );

  const renderQuickActionModal = () => {
    const isSQLInsert = quickAction === 'sql-insert';
    const isSQLUpdate = sqlRowAction === 'sql-update';
    const isRedisSet = quickAction === 'redis-set';
    const isRedisMemberAdd = quickAction === 'redis-member-add';
    const isRedisMemberUpdate = quickAction === 'redis-member-update';
    const isMongoInsert = quickAction === 'mongo-insert';
    const isMongoUpdate = quickAction === 'mongo-update';
    const redisType = objectDetail ? redisObjectType(objectDetail) : '';
    const columns = objectDetail ? sqlEditableColumns(objectDetail) : [];
    const identityColumn =
      target && objectDetail && sqlActionRow
        ? sqlIdentityColumn(target.protocol, objectDetail, sqlActionRow).column
        : '';
    const updateColumns = columns.filter(
      (column) => column.name !== identityColumn,
    );
    const title = isSQLInsert
      ? tr('新增行', 'Insert Row')
      : isSQLUpdate
      ? tr('更新行', 'Update Row')
      : isRedisSet
      ? tr('设置 Key', 'Set Key')
      : isRedisMemberUpdate
      ? tr('更新 Redis 成员', 'Update Redis Member')
      : isRedisMemberAdd
      ? tr('新增 Redis 成员', 'Add Redis Member')
      : isMongoUpdate
      ? tr('更新文档', 'Update Document')
      : tr('插入文档', 'Insert Document');

    return (
      <Modal
        title={title}
        open={Boolean(quickAction || sqlRowAction)}
        onCancel={closeQuickAction}
        onOk={() => quickForm.submit()}
        okText={tr('生成并执行', 'Build and Run')}
        cancelText={tr('取消', 'Cancel')}
        width={720}
        destroyOnClose
      >
        <Form
          form={quickForm}
          layout="vertical"
          onFinish={handleQuickActionSubmit}
        >
          {isSQLInsert && (
            <div className="webdata-quick-grid">
              {columns.map((column) => (
                <Form.Item
                  key={column.name}
                  name={['values', column.name]}
                  label={
                    column.type
                      ? `${column.name} · ${column.type}`
                      : column.name
                  }
                >
                  <Input
                    placeholder={tr('留空则不写入', 'Leave empty to skip')}
                  />
                </Form.Item>
              ))}
              {columns.length === 0 && (
                <Alert
                  type="warning"
                  showIcon
                  message={tr('没有可写入字段', 'No editable columns')}
                />
              )}
            </div>
          )}
          {isSQLUpdate && (
            <>
              <Alert
                type="info"
                showIcon
                className="webdata-alert"
                message={tr(
                  '将使用预览行的标识列生成 WHERE 条件。',
                  'The WHERE clause will use the selected row identity column.',
                )}
              />
              <div className="webdata-quick-grid">
                <Form.Item name="where_column" label="WHERE column">
                  <Input disabled />
                </Form.Item>
                <Form.Item name="where_value" label="WHERE value">
                  <Input disabled />
                </Form.Item>
                {updateColumns.map((column) => (
                  <Form.Item
                    key={column.name}
                    name={['values', column.name]}
                    label={
                      column.type
                        ? `${column.name} · ${column.type}`
                        : column.name
                    }
                  >
                    <Input
                      placeholder={tr(
                        '留空则不更新',
                        'Leave empty to skip update',
                      )}
                    />
                  </Form.Item>
                ))}
              </div>
            </>
          )}
          {isRedisSet && (
            <>
              <Form.Item
                name="key"
                label="Key"
                rules={[
                  { required: true, message: tr('请输入 Key', 'Enter key') },
                ]}
              >
                <Input />
              </Form.Item>
              <Form.Item name="value" label={tr('值', 'Value')}>
                <TextArea
                  autoSize={{ minRows: 4, maxRows: 10 }}
                  className="webdata-textarea"
                />
              </Form.Item>
              <Form.Item name="ttl" label="TTL (seconds)">
                <InputNumber min={1} precision={0} className="webdata-full" />
              </Form.Item>
            </>
          )}
          {(isRedisMemberAdd || isRedisMemberUpdate) && (
            <>
              <Alert
                type="info"
                showIcon
                className="webdata-alert"
                message={redisActionDescription(redisType)}
              />
              <Form.Item name="key" label="Key">
                <Input disabled />
              </Form.Item>
              {redisType === 'hash' && (
                <>
                  <Form.Item
                    name="field"
                    label={tr('字段', 'Field')}
                    rules={[
                      {
                        required: true,
                        message: tr('请输入字段', 'Enter field'),
                      },
                    ]}
                  >
                    <Input disabled={isRedisMemberUpdate} />
                  </Form.Item>
                  <Form.Item name="value" label={tr('值', 'Value')}>
                    <TextArea
                      autoSize={{ minRows: 4, maxRows: 10 }}
                      className="webdata-textarea"
                    />
                  </Form.Item>
                </>
              )}
              {redisType === 'list' && (
                <>
                  {isRedisMemberAdd ? (
                    <Form.Item name="direction" label={tr('方向', 'Side')}>
                      <Select
                        options={[
                          { label: 'RPUSH', value: 'RPUSH' },
                          { label: 'LPUSH', value: 'LPUSH' },
                        ]}
                      />
                    </Form.Item>
                  ) : (
                    <Form.Item name="index" label="Index">
                      <InputNumber disabled className="webdata-full" />
                    </Form.Item>
                  )}
                  <Form.Item name="value" label={tr('值', 'Value')}>
                    <TextArea
                      autoSize={{ minRows: 4, maxRows: 10 }}
                      className="webdata-textarea"
                    />
                  </Form.Item>
                </>
              )}
              {redisType === 'set' && (
                <Form.Item
                  name="member"
                  label={tr('成员', 'Member')}
                  rules={[
                    {
                      required: true,
                      message: tr('请输入成员', 'Enter member'),
                    },
                  ]}
                >
                  <TextArea
                    autoSize={{ minRows: 4, maxRows: 10 }}
                    className="webdata-textarea"
                  />
                </Form.Item>
              )}
              {redisType === 'zset' && (
                <>
                  <Form.Item
                    name="member"
                    label={tr('成员', 'Member')}
                    rules={[
                      {
                        required: true,
                        message: tr('请输入成员', 'Enter member'),
                      },
                    ]}
                  >
                    <Input disabled={isRedisMemberUpdate} />
                  </Form.Item>
                  <Form.Item
                    name="score"
                    label="Score"
                    rules={[
                      {
                        required: true,
                        message: tr('请输入分数', 'Enter score'),
                      },
                    ]}
                  >
                    <InputNumber className="webdata-full" />
                  </Form.Item>
                </>
              )}
              {redisType === 'stream' && isRedisMemberAdd && (
                <>
                  <Form.Item
                    name="field"
                    label={tr('字段', 'Field')}
                    rules={[
                      {
                        required: true,
                        message: tr('请输入字段', 'Enter field'),
                      },
                    ]}
                  >
                    <Input />
                  </Form.Item>
                  <Form.Item name="value" label={tr('值', 'Value')}>
                    <TextArea
                      autoSize={{ minRows: 4, maxRows: 10 }}
                      className="webdata-textarea"
                    />
                  </Form.Item>
                </>
              )}
            </>
          )}
          {(isMongoInsert || isMongoUpdate) && (
            <>
              {isMongoUpdate && (
                <>
                  <Alert
                    type="info"
                    showIcon
                    className="webdata-alert"
                    message={tr(
                      '将使用预览文档的 _id 生成过滤条件。',
                      'The filter will use the selected document _id.',
                    )}
                  />
                  <Form.Item name="filter" label="Filter">
                    <TextArea
                      readOnly
                      autoSize={{ minRows: 3, maxRows: 6 }}
                      className="webdata-textarea"
                    />
                  </Form.Item>
                </>
              )}
              <Form.Item
                name="document"
                label={
                  isMongoUpdate
                    ? tr('$set JSON', '$set JSON')
                    : tr('文档 JSON', 'Document JSON')
                }
                rules={[
                  {
                    required: true,
                    message: tr('请输入 JSON 文档', 'Enter a JSON document'),
                  },
                ]}
              >
                <TextArea
                  autoSize={{ minRows: 8, maxRows: 18 }}
                  className="webdata-textarea"
                />
              </Form.Item>
            </>
          )}
        </Form>
      </Modal>
    );
  };

  const renderObjectDetail = () => {
    if (!selectedNode) {
      return (
        <div className="webdata-empty">
          {tr('选择左侧对象查看详情', 'Select an object to inspect')}
        </div>
      );
    }
    if (objectLoading) {
      return (
        <div className="webdata-empty">
          <Spin />
        </div>
      );
    }
    if (!objectDetail) {
      return (
        <div className="webdata-empty">
          {tr('该节点没有对象详情', 'No object details for this node')}
        </div>
      );
    }
    const protocol = target?.protocol || '';
    const isSQLTable =
      isSQLProtocol(protocol) && objectDetail.object_type === 'table';
    const isRedisKey =
      protocol === 'redis' && objectDetail.object_type === 'key';
    const isMongoCollection =
      protocol === 'mongodb' && objectDetail.object_type === 'collection';
    const detailItems = [
      {
        key: 'type',
        label: tr('类型', 'Type'),
        children: <Tag>{objectDetail.object_type}</Tag>,
      },
      objectDetail.database
        ? {
            key: 'database',
            label: tr('数据库', 'Database'),
            children: objectDetail.database,
          }
        : undefined,
      objectDetail.schema
        ? { key: 'schema', label: 'Schema', children: objectDetail.schema }
        : undefined,
      objectDetail.name
        ? {
            key: 'name',
            label: tr('名称', 'Name'),
            children: objectDetail.name,
          }
        : undefined,
      objectDetail.key
        ? { key: 'key', label: 'Key', children: objectDetail.key }
        : undefined,
      objectDetail.message
        ? {
            key: 'message',
            label: tr('状态', 'Status'),
            children: objectDetail.message,
          }
        : undefined,
    ].filter(Boolean) as any[];
    const tabItems = [
      isSQLTable && Boolean(objectDetail.columns?.length)
        ? {
            key: 'columns',
            label: tr('字段', 'Columns'),
            children: renderMapTable(
              objectDetail.columns,
              tr('暂无字段信息', 'No columns'),
            ),
          }
        : undefined,
      (isSQLTable || isMongoCollection) && Boolean(objectDetail.indexes?.length)
        ? {
            key: 'indexes',
            label: tr('索引', 'Indexes'),
            children: renderMapTable(
              objectDetail.indexes,
              tr('暂无索引信息', 'No indexes'),
            ),
          }
        : undefined,
      (isRedisKey || isMongoCollection) && Boolean(objectDetail.extra?.length)
        ? {
            key: 'extra',
            label: tr('属性', 'Properties'),
            children: renderMapTable(
              objectDetail.extra,
              tr('暂无属性信息', 'No properties'),
            ),
          }
        : undefined,
      isSQLTable && Boolean(objectDetail.ddl)
        ? {
            key: 'ddl',
            label: 'DDL',
            children: (
              <TextArea
                readOnly
                value={objectDetail.ddl}
                autoSize={{ minRows: 8, maxRows: 18 }}
                className="webdata-textarea"
              />
            ),
          }
        : undefined,
    ].filter(Boolean) as any[];
    const actionButtons = [
      {
        key: 'sql-insert-row',
        show: isSQLTable,
        node: (
          <Button
            icon={<PlusOutlined />}
            size="small"
            onClick={() => openQuickAction('sql-insert')}
          >
            {tr('新增行', 'Insert Row')}
          </Button>
        ),
      },
      {
        key: 'sql-ddl',
        show: isSQLTable && Boolean(objectDetail.ddl),
        node: (
          <Button
            icon={<FileTextOutlined />}
            size="small"
            onClick={() => applyObjectTemplate('ddl')}
          >
            DDL
          </Button>
        ),
      },
      {
        key: 'redis-set',
        show: isRedisKey,
        node: (
          <Button
            icon={<EditOutlined />}
            size="small"
            onClick={() => openQuickAction('redis-set')}
          >
            {tr('设置值', 'Set Value')}
          </Button>
        ),
      },
      {
        key: 'redis-member-add',
        show: isRedisKey && canAddRedisMember(objectDetail),
        node: (
          <Button
            icon={<PlusOutlined />}
            size="small"
            onClick={() => openQuickAction('redis-member-add')}
          >
            {tr('新增成员', 'Add Member')}
          </Button>
        ),
      },
      {
        key: 'redis-delete',
        show: isRedisKey,
        node: (
          <Button
            danger
            icon={<DeleteOutlined />}
            size="small"
            onClick={deleteRedisKey}
          >
            {tr('删除 Key', 'Delete Key')}
          </Button>
        ),
      },
      {
        key: 'mongo-insert-doc',
        show: isMongoCollection,
        node: (
          <Button
            icon={<PlusOutlined />}
            size="small"
            onClick={() => openQuickAction('mongo-insert')}
          >
            {tr('插入文档', 'Insert Document')}
          </Button>
        ),
      },
    ];
    const objectLabel =
      objectDetail.name || objectDetail.key || selectedNode.title || '-';

    return (
      <Space direction="vertical" size={12} className="webdata-inspector-body">
        <div className="webdata-object-summary">
          <div>
            <Text type="secondary">{tr('当前对象', 'Current object')}</Text>
            <div className="webdata-object-name">{objectLabel}</div>
          </div>
          <Tag>{objectDetail.object_type}</Tag>
        </div>
        <Descriptions size="small" column={1} items={detailItems} />
        <Space wrap className="webdata-object-actions">
          {actionButtons
            .filter((item) => item.show)
            .map((item) => (
              <span key={item.key}>{item.node}</span>
            ))}
        </Space>
        {tabItems.length ? <Tabs size="small" items={tabItems} /> : null}
      </Space>
    );
  };

  const renderObjectDetailDrawer = () => (
    <Drawer
      className="webdata-inspector-drawer"
      title={tr('对象详情', 'Object Detail')}
      width="min(1180px, calc(100vw - 56px))"
      open={objectDetailOpen}
      onClose={() => setObjectDetailOpen(false)}
      destroyOnClose={false}
      extra={
        objectDetail ? (
          <Space size={8} wrap>
            <Tag>{objectDetail.object_type}</Tag>
            <Text type="secondary">
              {objectDetail.name ||
                objectDetail.key ||
                selectedNode?.title ||
                '-'}
            </Text>
          </Space>
        ) : undefined
      }
    >
      <div className="webdata-inspector-drawer-body">
        {renderObjectDetail()}
      </div>
    </Drawer>
  );

  if (loading) {
    return (
      <PageContainer title={false}>
        <div className="webdata-loading">
          <Spin />
        </div>
      </PageContainer>
    );
  }

  return (
    <PageContainer title={false}>
      <div className="webdata-shell">
        <div className="webdata-header">
          <div className="webdata-header-main">
            <DatabaseOutlined className="webdata-header-icon" />
            <div className="webdata-header-copy">
              <Text type="secondary">
                {tr('网页数据控制台', 'Web Data Console')}
              </Text>
              <Title level={4} className="webdata-title">
                {target?.proxy_name || tr('数据控制台', 'Data Console')}
              </Title>
              <Space size={8} wrap className="webdata-header-meta">
                <CloudServerOutlined />
                <Text type="secondary">{target?.application_name}</Text>
                <Text type="secondary">
                  {target?.target_host}:{target?.target_port}
                </Text>
              </Space>
            </div>
          </div>
          <div className="webdata-header-actions">
            <Space wrap>
              {target && (
                <Tag color="processing">
                  {protocolLabels[target.protocol] || target.protocol}
                </Tag>
              )}
              {connected && session?.expires_at && (
                <Tag color="success" icon={<CheckCircleOutlined />}>
                  {tr('会话有效', 'Session active')}
                </Tag>
              )}
              {target && (
                <Button
                  icon={<AuditOutlined />}
                  onClick={() => history.push(`/audit?proxy_id=${proxyId}`)}
                >
                  {tr('审计', 'Audit')}
                </Button>
              )}
              {connected ? (
                <>
                  <Button
                    icon={<DisconnectOutlined />}
                    onClick={handleDisconnect}
                  >
                    {tr('断开', 'Disconnect')}
                  </Button>
                </>
              ) : (
                <Button onClick={() => history.push('/proxy')}>
                  {tr('返回访问', 'Back')}
                </Button>
              )}
            </Space>
          </div>
        </div>

        {target?.effective_status !== 'active' && (
          <Alert
            type="warning"
            showIcon
            className="webdata-alert"
            message={
              target?.effective_status_message ||
              tr('目标不可用', 'Target unavailable')
            }
          />
        )}

        {!connected ? (
          renderConnectionManager()
        ) : (
          <div className="webdata-workbench">
            <div className="webdata-side-frame">
              <aside className="webdata-side">
                <div className="webdata-side-header">
                  <Space>
                    <TableOutlined />
                    <Text strong>{tr('对象', 'Objects')}</Text>
                  </Space>
                  <Tooltip title={tr('刷新对象', 'Refresh objects')}>
                    <Button
                      icon={<ReloadOutlined />}
                      size="small"
                      loading={metadataLoading}
                      onClick={() => loadMetadata()}
                    />
                  </Tooltip>
                </div>
                <Input
                  allowClear
                  size="small"
                  prefix={<SearchOutlined />}
                  className="webdata-object-search"
                  placeholder={tr('搜索库、表、字段', 'Search objects')}
                  value={treeSearch}
                  onChange={(event) => setTreeSearch(event.target.value)}
                />
                <div className="webdata-object-stats">
                  <span>
                    <strong>{metadataSummary.databases}</strong>
                    {tr('库', 'DB')}
                  </span>
                  <span>
                    <strong>
                      {metadataSummary.tables +
                        metadataSummary.collections +
                        metadataSummary.keys}
                    </strong>
                    {tr('对象', 'Objects')}
                  </span>
                  <span>
                    <strong>{metadataSummary.columns}</strong>
                    {tr('字段', 'Fields')}
                  </span>
                </div>
                <div className="webdata-object-tree">
                  <Spin spinning={metadataLoading}>
                    {treeData.length ? (
                      <Tree
                        key={treeSearch ? `search-${treeSearch}` : 'objects'}
                        treeData={treeData}
                        defaultExpandAll={Boolean(treeSearch)}
                        blockNode
                        selectedKeys={selectedNode ? [selectedNode.key] : []}
                        onSelect={handleTreeSelect}
                      />
                    ) : (
                      <Empty
                        image={Empty.PRESENTED_IMAGE_SIMPLE}
                        description={tr('没有匹配对象', 'No matching objects')}
                      />
                    )}
                  </Spin>
                </div>
              </aside>
            </div>

            <main className="webdata-main">
              <div className="webdata-editor">
                <div className="webdata-editor-toolbar">
                  <div className="webdata-editor-title">
                    <Space>
                      <ApiOutlined />
                      <Text strong>{tr('命令窗口', 'Command Window')}</Text>
                    </Space>
                    <Text type="secondary">
                      {target
                        ? protocolLabels[target.protocol] || target.protocol
                        : '-'}
                    </Text>
                  </div>
                  <Space wrap className="webdata-editor-actions">
                    {isSQLProtocol(target?.protocol) && (
                      <Button
                        icon={<FileSearchOutlined />}
                        loading={executing}
                        onClick={handleExplainStatement}
                      >
                        Explain
                      </Button>
                    )}
                    <Button
                      icon={<CopyOutlined />}
                      onClick={copyCurrentStatement}
                    >
                      {tr('复制', 'Copy')}
                    </Button>
                    <Button
                      icon={<FileTextOutlined />}
                      disabled={!selectedNode}
                      onClick={() => setObjectDetailOpen(true)}
                    >
                      {tr('对象详情', 'Details')}
                    </Button>
                    <Tooltip
                      title={tr(
                        '有选中文本时只执行选中命令',
                        'Runs only the selected command when text is selected',
                      )}
                    >
                      <Button
                        type="primary"
                        icon={<PlayCircleOutlined />}
                        loading={executing}
                        onClick={runStatement}
                      >
                        {tr('执行', 'Run')}
                      </Button>
                    </Tooltip>
                  </Space>
                </div>
                {objectDetail &&
                  ((isSQLProtocol(target?.protocol) &&
                    objectDetail.object_type === 'table') ||
                    (target?.protocol === 'mongodb' &&
                      objectDetail.object_type === 'collection')) &&
                  renderObjectFilter(
                    objectFilterFieldInfos(target?.protocol, objectDetail),
                  )}
                <div className="webdata-editor-input webdata-code-editor">
                  <div className="webdata-editor-gutter" aria-hidden="true">
                    <div
                      className="webdata-editor-gutter-lines"
                      style={{
                        transform: `translateY(${-editorScrollTop}px)`,
                      }}
                    >
                      {editorLineNumbers.map((line) => (
                        <span key={line}>{line}</span>
                      ))}
                    </div>
                  </div>
                  <div
                    className="webdata-editor-active-line"
                    style={{
                      transform: `translateY(${
                        (editorActiveLine - 1) * 22 - editorScrollTop
                      }px)`,
                    }}
                  />
                  <pre
                    className="webdata-editor-highlight"
                    aria-hidden="true"
                    style={{
                      transform: `translateY(${-editorScrollTop}px)`,
                    }}
                  >
                    {renderStatementHighlight(statement, target?.protocol)}
                  </pre>
                  <TextArea
                    ref={statementTextAreaRef}
                    value={statement}
                    onChange={handleStatementChange}
                    onFocus={() => setCompletionActive(true)}
                    onBlur={() => {
                      window.setTimeout(() => {
                        const textarea = getStatementTextArea(
                          statementTextAreaRef.current,
                        );
                        if (textarea && document.activeElement === textarea) {
                          return;
                        }
                        setCompletionActive(false);
                        setCompletionForced(false);
                      }, 120);
                    }}
                    onClick={updateEditorCursor}
                    onKeyUp={updateEditorCursor}
                    onKeyDown={handleStatementKeyDown}
                    onScroll={handleStatementScroll}
                    placeholder={statementPlaceholder(target?.protocol, tr)}
                    spellCheck={false}
                    autoSize={{ minRows: 10, maxRows: 20 }}
                    className="webdata-textarea webdata-statement-textarea"
                  />
                  {completionVisible && (
                    <div
                      className="webdata-completion"
                      style={{
                        left: completionPosition.left,
                        top: completionPosition.top,
                        width: completionPosition.width,
                        maxHeight: completionPosition.maxHeight,
                      }}
                      onMouseDown={(event) => event.preventDefault()}
                    >
                      {completionItems.map((item, index) => (
                        <button
                          key={item.key}
                          type="button"
                          className={`webdata-completion-item ${
                            index === completionIndex ? 'is-active' : ''
                          }`}
                          onMouseEnter={() => setCompletionIndex(index)}
                          onClick={() => applyCompletion(item)}
                        >
                          <span
                            className={`webdata-completion-icon is-${item.type}`}
                          >
                            {completionTypeIcon(item)}
                          </span>
                          <span className="webdata-completion-main">
                            <span className="webdata-completion-label">
                              {item.label}
                            </span>
                          </span>
                          {item.detail && (
                            <span className="webdata-completion-detail">
                              {item.detail}
                            </span>
                          )}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </div>

              <div className="webdata-result">
                <div className="webdata-result-meta">
                  <Space>
                    <ClusterOutlined />
                    <Text strong>{tr('执行结果', 'Execution Result')}</Text>
                  </Space>
                  {result && (
                    <Space size={8} wrap className="webdata-result-actions">
                      <Space size={8} wrap>
                        <Tag>
                          {resultSummary.rows} {tr('行', 'rows')}
                        </Tag>
                        <Tag>
                          {resultSummary.columns} {tr('列', 'cols')}
                        </Tag>
                        <Tag>{result.elapsed_ms || 0} ms</Tag>
                        {(isSQLProtocol(target?.protocol) ||
                          target?.protocol === 'mongodb' ||
                          Boolean(result.affected_rows)) && (
                          <Tag>
                            {result.affected_rows || 0}{' '}
                            {tr('影响行', 'affected')}
                          </Tag>
                        )}
                        {result.truncated && (
                          <Tag color="warning">{tr('已截断', 'Truncated')}</Tag>
                        )}
                      </Space>
                      {Boolean(result.rows?.length) && (
                        <Space size={4} wrap>
                          <Button
                            size="small"
                            icon={<DownloadOutlined />}
                            onClick={() => handleExportResult('csv')}
                          >
                            CSV
                          </Button>
                          <Button
                            size="small"
                            icon={<DownloadOutlined />}
                            onClick={() => handleExportResult('json')}
                          >
                            JSON
                          </Button>
                        </Space>
                      )}
                    </Space>
                  )}
                </div>
                {renderRowsTable(result, tr('暂无结果', 'No result'), {
                  ...currentObjectRowActionOptions(),
                  tableId: 'execution-result',
                })}
              </div>
            </main>
          </div>
        )}
        {renderAuditDrawer()}
        {renderConnectionDrawer()}
        {renderQuickActionModal()}
        {renderObjectDetailDrawer()}
      </div>
    </PageContainer>
  );
};

export default WebDataPage;
