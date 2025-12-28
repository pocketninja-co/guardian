<script setup>
import { ref, reactive, onMounted, computed, nextTick } from 'vue'
import { NConfigProvider, NGlobalStyle, NMessageProvider, lightTheme, NSwitch, NInputNumber, NTooltip } from 'naive-ui'
import { 
  ShieldCheck, AlertTriangle, FileText, Activity, 
  FolderSearch, UploadCloud, XCircle, Download, 
  Home, Settings, User, PieChart, BarChart, TrendingUp,
  Search, Bell, Lock, Info, Sparkles, Trash2, CheckCircle, HelpCircle
} from 'lucide-vue-next'
import logoUrl from './assets/logo.svg'

// Mocking the Wails backend calls
const backendCalls = window.go?.main?.App || { 
  AnalyzeFile: async (path) => {
    return { RiskScore: 85, Findings: ["Simulated SSN found on line 12", "Credit Card Pattern detected on line 45"], IsClean: false, Certificate: "" } 
  },
  CancelScan: async () => {},
  GenerateReport: async () => {},
  SelectDirectory: async () => "/simulated/path",
  ScanDirectory: async () => ({
      totalFiles: 1243, totalRiskScore: 50, criticalCount: 12, potentialLiability: 250000, 
      topOffenders: []
  }),
  SelectFile: async () => "test.txt",
  SelectFiles: async () => ["test1.txt", "test2.log"],
  RedactFile: async () => "test_CLEANED.txt",
  UpdateStats: async () => {},
  RunNow: async () => {},
  CancelScheduledScan: async () => { console.log('Scan cancelled!') },
  OpenPath: async (path) => { console.log('Opening path:', path) },
  GetScheduleConfig: async () => ({ schedule_enabled: false, scan_interval_hours: 24, scan_paths: [], audit_history: [], total_files_scanned: 0, total_risks_found: 0, total_liability: 0 })
};

// Application State
const currentView = ref('overview') 
const logs = ref([])
const stats = reactive({
  scanned: 0,
  blocked: 0,
  liability: 0,
  status: 'Ready'
})
const settings = reactive({
    deepScan: true,
    autoReport: false,
    riskThreshold: 80,
    notificationSound: true
})
const currentUser = ref('Loading...')

// Load real username from backend
if (window.go?.main?.App) {
  window.go.main.App.GetUsername().then(username => {
    currentUser.value = username || 'User'
  }).catch(() => {
    currentUser.value = 'User'
  })
} else {
  currentUser.value = 'Demo User'
}

// Batch Queue State
const fileQueue = ref([]) // { path: string, status: 'pending'|'scanning'|'clean'|'risk'|'sanitized', riskScore: number, error: string, findings: [], selected: boolean }

// Selection & Pagination
const currentPage = ref(1)
const pageSize = ref(25)
const selectAll = ref(false)

const isDragging = ref(false)
const isScanning = ref(false)
const isGeneratingReport = ref(false)
const currentReport = ref(null)
// Analytics
const scanTrend = ref([])
const riskDistribution = ref([])

// Check for recent clean audit
const latestCleanAudit = computed(() => {
    if (!scheduleConfig.value?.audit_history) return null
    // Sort by timestamp desc
    const sorted = [...scheduleConfig.value.audit_history].sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp))
    return sorted.find(entry => entry.status === 'PASSED')
})

const scanMode = ref("file") 
const showScanOverlay = ref(false)
const auditSearch = ref("")

// Schedule Configuration State
const scheduleConfig = ref({
  schedule_enabled: false,
  scan_interval_hours: 24,
  interval_value: 1,
  interval_unit: 'days',
  time_of_day: '02:00',
  timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  scan_paths: [],
  audit_history: []
})
const isLoadingSchedule = ref(false)
const isScheduledScanning = ref(false)
const scanProgress = ref({ current: 0, total: 0, file: '' })

// fake delay for cinematic effect
const wait = (ms) => new Promise(resolve => setTimeout(resolve, ms))

const addLog = (msg, type = 'info') => {
  const timestamp = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  logs.value.unshift({ time: timestamp, msg, type })
}

const stopScan = async () => {
    try {
        await backendCalls.CancelScan()
        addLog("Scan stopped by user.", 'warning')
        showScanOverlay.value = false
        isScanning.value = false
    } catch(e) { addLog(`Error stopping: ${e}`, 'error') }
}

const generateReport = async () => {
  addLog('Generating report...', 'info')
  isGeneratingReport.value = true
  
  // If we have a certificate path directly (from auto-scan)
  if (currentReport.value?.certificate) {
      await backendCalls.OpenPath(currentReport.value.certificate)
      isGeneratingReport.value = false
      return
  }

  // Use current report OR fallback to latest clean audit
  const reportData = currentReport.value ? {
    TotalFiles: currentReport.value.totalFiles,
    TotalRiskScore: currentReport.value.totalRiskScore,
    CriticalCount: currentReport.value.criticalCount || 0,
    PotentialLiability: currentReport.value.potentialLiability || 0,
    TopOffenders: [] // Full list not kept in frontend for simplicity often
  } : (latestCleanAudit.value ? {
      TotalFiles: latestCleanAudit.value.total_files,
      TotalRiskScore: 0, // It's clean
      CriticalCount: 0,
      PotentialLiability: 0,
      TopOffenders: []
  } : null)

  if (!reportData) {
      addLog("No report data available.", "error")
      isGeneratingReport.value = false
      return
  }

  try {
    const path = await backendCalls.GenerateReport(reportData)
    addLog(`Report saved to: ${path.split(/[\\/]/).pop()}`, 'success')
    await backendCalls.OpenPath(path)
  } catch(e) {
    if (e.includes("cancelled")) {
        addLog("Report generation cancelled by user", 'info')
    } else {
        addLog(`Report generation failed: ${e}`, 'error')
    }
  } finally {
    isGeneratingReport.value = false
  }
}

// Batch Logic
const addToQueue = async (paths) => {
    if (!paths) return
    for (const path of paths) {
        if (!fileQueue.value.find(f => f.path === path)) {
            fileQueue.value.push({ path, status: 'pending', riskScore: 0, findings: [] })
        }
    }
    processQueue()
}

const processQueue = async () => {
    if (isScanning.value) return
    isScanning.value = true
    
    // Find next pending
    const pendingIndex = fileQueue.value.findIndex(f => f.status === 'pending')
    if (pendingIndex === -1) {
        isScanning.value = false
        showScanOverlay.value = false
        return
    }

    const file = fileQueue.value[pendingIndex]
    file.status = 'scanning'
    
    try {
        addLog(`Analyzing: ${file.path.split(/[\\/]/).pop()}`, 'info')
        const [result] = await Promise.all([
            backendCalls.AnalyzeFile(file.path),
            wait(500) // faster for batch
        ])
        const res = await backendCalls.AnalyzeFile(file.path)
        file.riskScore = res.RiskScore || res.riskScore || 0
        file.findings = res.Findings || res.findings || []
        file.status = file.riskScore > 0 ? 'risk' : 'clean'
        
        // Update stats locally (will persist via UpdateStats call)
        stats.scanned++
        if (file.riskScore > 0) {
            stats.blocked++
            const liability = file.riskScore > 50 ? 5000 : 1000
            stats.liability += liability
            
            // Persist to backend
            if (backendCalls.UpdateStats) {
                backendCalls.UpdateStats(1, 1, liability).catch(() => {})
            }
            
            addLog(`Risk Detected: ${file.path.split(/[\\/]/).pop()} (Score: ${file.riskScore})`, 'warning')
        } else {
            // Persist clean file scan
            if (backendCalls.UpdateStats) {
                backendCalls.UpdateStats(1, 0, 0).catch(() => {})
            }
            addLog(`Clean: ${file.path.split(/[\\/]/).pop()}`, 'success')
        }
    } catch(err) {
        file.status = 'error'
        file.error = err.toString()
        addLog(`Error scanning ${file.path}: ${err}`, 'error')
    }
    
    // Recursively process next
    isScanning.value = false
    processQueue()
}

const bulkSanitize = async () => {
    const riskyFiles = fileQueue.value.filter(f => f.status === 'risk')
    if (riskyFiles.length === 0) return
    
    addLog(`Starting Bulk Sanitization for ${riskyFiles.length} files...`, 'info')
    
    for (const file of riskyFiles) {
        await sanitizeSingle(file)
        await wait(300)
    }
}

const sanitizeSingle = async (file) => {
    try {
        addLog(`Sanitizing: ${file.path.split(/[\\/]/).pop()}`, 'info')
        const cleanPath = await backendCalls.RedactFile(file.path)
        file.status = 'sanitized'
        
        // Reduce liability since risk is now mitigated
        const liability = file.riskScore > 50 ? 5000 : 1000
        stats.liability = Math.max(0, stats.liability - liability)
        stats.blocked = Math.max(0, stats.blocked - 1)
        
        // Update backend
        if (backendCalls.UpdateStats) {
            backendCalls.UpdateStats(0, -1, -liability).catch(() => {})
        }
        
        addLog(`Sanitized: ${cleanPath.split(/[\\/]/).pop()}`, 'success')
    } catch (e) {
        addLog(`Error sanitizing: ${e}`, 'error')
    }
}

const removeFile = (index) => {
    fileQueue.value.splice(index, 1)
}

const scanDirectory = async () => {
    try {
        const path = await backendCalls.SelectDirectory()
        if (path) {
            stats.status = "Scanning"
            isScanning.value = true
            showScanOverlay.value = true
            currentReport.value = null
            addLog(`Deep Audit initialized: ${path}`, 'info')
            
            const [report] = await Promise.all([
                backendCalls.ScanDirectory(path),
                wait(2000) 
            ])
            
            isScanning.value = false
            setTimeout(() => { showScanOverlay.value = false }, 500)
            
            currentReport.value = report 
            
            if (!report) return
            
            stats.scanned += report.totalFiles
            if (report.totalRiskScore > 0) {
                 stats.blocked += report.criticalCount
                 stats.status = "Risks Found"
                 stats.liability += report.potentialLiability
            } else {
                stats.status = "Secure"
            }
        }
    } catch(e) {
        isScanning.value = false
        showScanOverlay.value = false
        addLog(`Scan Error: ${e}`, 'error')
    }
}

const selectFiles = async () => {
    if (scanMode.value === 'directory') {
        scanDirectory()
        return
    }
    try {
        const paths = await backendCalls.SelectFiles()
        if (paths && paths.length > 0) addToQueue(paths)
    } catch(e) { addLog(`Selection Error: ${e}`, 'error') }
}

const onDragEnter = (e) => { e.preventDefault(); isDragging.value = true }
const onDragLeave = (e) => { e.preventDefault(); isDragging.value = false }
const onDrop = async (e) => { 
    e.preventDefault(); 
    isDragging.value = false 
} 

onMounted(() => {
  loadScheduleConfig()
  
  if (window.runtime) {
    window.runtime.EventsOn("scan:dir_start", (path) => addLog(`Scanning: ${path}`, 'info'))
    window.runtime.EventsOn("scan:cancelled", (msg) => {
        addLog(msg, 'warning')
        isScanning.value = false
        showScanOverlay.value = false
        stats.status = 'Stopped'
    })
    
    // Wails Runtime Drop
    if (window.runtime.OnFileDrop) {
      window.runtime.OnFileDrop((x, y, paths) => {
        if (paths && paths.length > 0) addToQueue(paths)
      })
    }
    
    // Scheduled scan events
    window.runtime.EventsOn("scan:scheduled:start", (data) => {
      isScheduledScanning.value = true
      scanProgress.value = { current: 0, total: data?.total || 0, file: '' }
      const pathCount = data?.paths?.length || 0
      addLog(`Scan started: ${data?.total || 0} files in ${pathCount} folder(s)`, 'info')
    })
    window.runtime.EventsOn("scan:scheduled:progress", (data) => {
      scanProgress.value = {
        current: data?.current || 0,
        total: data?.total || 0,
        file: data?.file || ''
      }
    })
    window.runtime.EventsOn("scan:scheduled:complete", (data) => {
      isScheduledScanning.value = false
      scanProgress.value = { current: 0, total: 0, file: '' }
      
      if (data.status === 'PASSED' && data.certificate) {
          addLog(`Compliance Certificate Generated`, 'success')
          
          // Let's store the cert path to open it
          currentReport.value = {
              totalFiles: data.total_files,
              totalRiskScore: 0,
              certificate: data.certificate
          }
      } else {
        addLog(`Scan complete: ${data.total_files} files, ${data.status === 'PASSED' ? '0 risks' : data.risk_score + ' risk score'}`, data.status === 'PASSED' ? 'success' : 'warning')
      }
      
      // Add risky files to queue for user to fix
      if (data.risky_files && data.risky_files.length > 0) {
        for (const file of data.risky_files) {
          // Check if file already in queue
          const exists = fileQueue.value.some(f => f.path === file.path)
          if (!exists) {
            fileQueue.value.push({
              path: file.path,
              status: 'risk',
              riskScore: file.riskScore,
              findings: file.findings || [],
              selected: false
            })
          }
        }
        addLog(`Added ${data.risky_files.length} risky files to queue for review`, 'warning')
        currentView.value = 'overview' // Switch to overview to show files
      }
      
      loadScheduleConfig() // Refresh audit history
    })
    window.runtime.EventsOn("scan:scheduled:error", (data) => {
      isScheduledScanning.value = false
      scanProgress.value = { current: 0, total: 0, file: '' }
      addLog(`Scan error: ${data.error}`, 'error')
    })
    
    // Backend Log Events
    window.runtime.EventsOn("log:info", (message) => {
      addLog(message, 'info')
    })
  }
})

const filteredLogs = computed(() => {
    if (!auditSearch.value) return logs.value
    return logs.value.filter(l => l.msg.toLowerCase().includes(auditSearch.value.toLowerCase()))
})
const riskyFilesCount = computed(() => fileQueue.value.filter(f => f.status === 'risk').length)

// Pagination computed properties
const totalPages = computed(() => Math.ceil(fileQueue.value.length / pageSize.value) || 1)
const paginatedFiles = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value
    return fileQueue.value.slice(start, start + pageSize.value)
})
const selectedCount = computed(() => fileQueue.value.filter(f => f.selected).length)
const selectedRiskyFiles = computed(() => fileQueue.value.filter(f => f.selected && f.status === 'risk'))

// Selection methods
const toggleSelectAll = () => {
    const newValue = !selectAll.value
    selectAll.value = newValue
    fileQueue.value.forEach(f => f.selected = newValue)
}
const toggleFileSelection = (file) => {
    file.selected = !file.selected
    selectAll.value = fileQueue.value.every(f => f.selected)
}
const bulkSanitizeSelected = async () => {
    const toSanitize = selectedRiskyFiles.value
    if (toSanitize.length === 0) {
        addLog('No risky files selected for sanitization', 'warning')
        return
    }
    addLog(`Sanitizing ${toSanitize.length} selected files...`, 'info')
    for (const file of toSanitize) {
        await sanitizeSingle(file)
        await wait(300)
    }
    addLog(`Bulk sanitization complete`, 'success')
}

// Clean files management
const cleanFilesCount = computed(() => fileQueue.value.filter(f => f.status === 'clean' || f.status === 'sanitized').length)
const clearAllClean = () => {
    fileQueue.value = fileQueue.value.filter(f => f.status !== 'clean' && f.status !== 'sanitized')
    addLog(`Cleared ${cleanFilesCount.value} clean/sanitized files from queue`, 'success')
    currentPage.value = 1 // Reset to first page
}

// Schedule Configuration Methods
const loadScheduleConfig = async () => {
  try {
    const config = await backendCalls.GetScheduleConfig()
    scheduleConfig.value = config
    
    // Update stats from persistent storage
    if (config) {
      stats.scanned = config.total_files_scanned || 0
      stats.blocked = config.total_risks_found || 0
      stats.liability = config.total_liability || 0
    }
  } catch(e) {
    addLog(`Failed to load schedule: ${e}`, 'error')
  }
}

const saveScheduleConfig = async () => {
  try {
    await backendCalls.UpdateScheduleConfig(scheduleConfig.value)
    addLog('Schedule settings saved', 'success')
  } catch(e) {
    addLog(`Failed to save schedule: ${e}`, 'error')
  } finally {
    isLoadingSchedule.value = false
  }
}

const triggerScanNow = async () => {
  if (isScheduledScanning.value) return // Prevent double-click
  
  try {
    isScheduledScanning.value = true
    addLog('Starting scheduled scan...', 'info')
    await backendCalls.TriggerScheduledScan()
    // Note: scan runs async, isScheduledScanning will be set to false by event listener
  } catch(e) {
    isScheduledScanning.value = false
    addLog(`Failed to trigger scan: ${e}`, 'error')
  }
}

const cancelScheduledScan = async () => {
    try {
        addLog('Cancelling scheduled scan...', 'info')
        await backendCalls.CancelScheduledScan()
    } catch(e) {
        addLog(`Failed to cancel scan: ${e}`, 'error')
    }
}

const addSchedulePath = async () => {
  try {
    const path = await backendCalls.SelectDirectory()
    if (path && !scheduleConfig.value.scan_paths.includes(path)) {
      await backendCalls.AddSchedulePath(path)
      await loadScheduleConfig()
      addLog(`Added folder to schedule: ${path.split(/[\\/]/).pop()}`, 'success')
    }
  } catch(e) {
    addLog(`Failed to add path: ${e}`, 'error')
  }
}

const removeSchedulePath = async (path) => {
  try {
    await backendCalls.RemoveSchedulePath(path)
    await loadScheduleConfig()
    addLog(`Removed folder from schedule`, 'success')
  } catch(e) {
    addLog(`Failed to remove path: ${e}`, 'error')
  }
}
</script>

<template>
  <n-config-provider :theme="lightTheme">
    <n-global-style />
    <n-message-provider>
      <div 
        class="flex h-screen bg-muted text-primary font-sans overflow-hidden"
        @dragenter.prevent="onDragEnter" 
        @dragover.prevent 
        @dragleave.prevent="onDragLeave"
        @drop="onDrop"
      >
        <!-- Titlebar Drag Area -->
        <div class="fixed top-0 left-0 right-0 h-8 z-50 pointer-events-none" style="--wails-draggable:drag"></div>

        <!-- Sidebar -->
        <aside class="w-64 bg-surface border-r border-border flex flex-col z-20 pt-8" style="--wails-draggable:drag">
            <div class="px-6 py-4 flex items-center gap-3" style="--wails-draggable:no-drag">
                <img :src="logoUrl" alt="Guardian Logo" class="w-6 h-6 rounded-lg shadow-lg" />
                <span class="font-bold text-lg tracking-tight">Guardian</span>
            </div>
            
            <nav class="flex-1 px-4 py-6 space-y-1" style="--wails-draggable:no-drag">
                <button @click="currentView = 'overview'" 
                    class="w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-md transition-colors text-left"
                    :class="currentView === 'overview' ? 'bg-primary text-white shadow-md' : 'text-primary-light hover:bg-muted hover:text-primary'"
                >
                    <Home :size="18" /> 
                    <span>Overview</span>
                </button>
                <button @click="currentView = 'analytics'"
                    class="w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-md transition-colors text-left"
                    :class="currentView === 'analytics' ? 'bg-primary text-white shadow-md' : 'text-primary-light hover:bg-muted hover:text-primary'"
                >
                    <PieChart :size="18" /> 
                    <span>Analytics</span>
                </button>
                <button @click="currentView = 'audit'"
                    class="w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-md transition-colors text-left"
                    :class="currentView === 'audit' ? 'bg-primary text-white shadow-md' : 'text-primary-light hover:bg-muted hover:text-primary'"
                >
                    <Activity :size="18" /> 
                    <span>Live Audit</span>
                </button>
                <button @click="currentView = 'settings'"
                    class="w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-md transition-colors text-left"
                    :class="currentView === 'settings' ? 'bg-primary text-white shadow-md' : 'text-primary-light hover:bg-muted hover:text-primary'"
                >
                    <Settings :size="18" /> 
                    <span>Settings</span>
                </button>
            </nav>

            <div class="p-4 border-t border-border/50" style="--wails-draggable:no-drag">
                <div class="flex items-center gap-3 px-3 py-2 rounded-md hover:bg-muted transition-colors cursor-pointer group relative">
                    <div class="w-8 h-8 rounded-full bg-indigo-100 flex items-center justify-center text-indigo-600 font-bold text-xs border border-indigo-200">
                        {{ currentUser.charAt(0).toUpperCase() }}
                    </div>
                    <div class="flex flex-col">
                        <span class="text-sm font-medium">{{ currentUser }}</span>
                        <div class="flex items-center gap-1">
                             <span class="text-[10px] text-primary-light">v1.2.0</span>
                             <span class="text-[10px] text-primary-light opacity-50">•</span>
                             <span class="text-[10px] text-primary-light">Open Source</span>
                        </div>
                    </div>
                    
                    <!-- Simple About Tooltip/Popover on Cover -->
                    <div class="absolute bottom-full left-0 w-48 mb-2 hidden group-hover:block bg-slate-800 text-white p-3 rounded shadow-xl text-xs z-50">
                        <p class="font-bold mb-1">Guardian v1.2.0</p>
                        <p class="opacity-80">HIPAA Compliance Tool</p>
                        <p class="opacity-80 mt-1">© 2025 Pocket Ninja LLC</p>
                    </div>
                </div>
            </div>
        </aside>

        <!-- Main Content -->
        <main class="flex-1 flex flex-col relative min-w-0 bg-muted/50">
            <!-- Scheduled Scan Progress Banner -->
            <div v-if="isScheduledScanning" class="bg-gradient-to-r from-blue-600 to-indigo-600 text-white px-6 py-3">
                <div class="flex items-center justify-between mb-2">
                    <div class="flex items-center gap-3">
                        <div class="w-3 h-3 bg-white rounded-full animate-pulse"></div>
                        <span class="font-medium">Scheduled Scan in Progress</span>
                        <span class="px-2 py-0.5 bg-white/20 rounded text-sm font-mono">
                            ({{ scanProgress.current }}/{{ scanProgress.total }})
                        </span>
                    </div>
                    <div class="flex items-center gap-2">
                        <span class="text-xs text-white/80 truncate max-w-[200px]">{{ scanProgress.file || 'Starting...' }}</span>
                        <!-- Cancel Button -->
                        <button 
                            @click="cancelScheduledScan"
                            class="px-2 py-0.5 rounded bg-white/20 hover:bg-white/30 text-xs font-semibold text-white transition-colors border border-white/30 whitespace-nowrap"
                        >
                            Cancel
                        </button>
                    </div>
                </div>
                <!-- Progress Bar -->
                <div class="h-1.5 bg-white/20 rounded-full overflow-hidden">
                    <div 
                        v-if="scanProgress.total > 0"
                        class="h-full bg-white rounded-full transition-all duration-300"
                        :style="{ width: (scanProgress.current / scanProgress.total * 100) + '%' }"
                    ></div>
                    <div 
                        v-else
                        class="h-full bg-white rounded-full animate-progress-indeterminate"
                    ></div>
                </div>
            </div>
            
            <!-- Header -->
            <header class="h-16 flex items-center justify-between px-8 pt-4 pb-2" style="--wails-draggable:drag">
                <div class="flex items-center gap-4" style="--wails-draggable:no-drag">
                     <h2 class="font-semibold text-xl tracking-tight capitalize">{{ currentView }}</h2>
                     <div class="h-4 w-px bg-border"></div>
                     <span class="text-sm text-primary-light">{{ new Date().toLocaleDateString() }}</span>
                </div>
                
                <!-- Actions -->
                <div class="flex items-center gap-3" style="--wails-draggable:no-drag">
                    
                    <button 
                         v-if="currentView === 'overview' && riskyFilesCount > 0"
                         @click="bulkSanitize"
                         class="group flex items-center gap-2 px-4 py-2 rounded-full text-xs font-bold bg-gradient-to-r from-violet-600 to-indigo-600 text-white shadow-md hover:shadow-lg hover:from-violet-500 hover:to-indigo-500 transition-all active:scale-95 border border-violet-400/50"
                    >
                        <Sparkles :size="14" class="group-hover:animate-spin-slow" /> 
                        Bulk Sanitize ({{ riskyFilesCount }})
                    </button>

                    <button 
                         v-if="currentView === 'overview' && !isScanning && (currentReport || latestCleanAudit)"
                         @click="generateReport"
                         :disabled="isGeneratingReport"
                         class="flex items-center gap-2 px-4 py-2 rounded-full text-xs font-bold bg-white border border-border text-primary shadow-sm hover:bg-gray-50 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                         :class="{'text-emerald-600 border-emerald-200': (currentReport?.totalRiskScore === 0 || latestCleanAudit)}"
                    >
                        <Download v-if="!isGeneratingReport" :size="14" />
                        <div v-else class="w-3.5 h-3.5 border-2 border-current border-t-transparent rounded-full animate-spin"></div>
                        {{ isGeneratingReport ? 'Saving...' : ((currentReport?.totalRiskScore === 0 || latestCleanAudit) ? 'Download Certificate' : 'Audit Report') }}
                    </button>
                    
                    <button 
                         v-if="currentView === 'overview'"
                        @click="selectFiles"
                        class="flex items-center gap-2 px-4 py-2 rounded-full text-xs font-bold bg-primary text-white shadow-lg hover:bg-primary/90 transition-transform active:scale-95"
                    >
                        <UploadCloud :size="14" /> 
                        {{ scanMode === 'directory' ? 'Scan Dir' : 'Add Files' }}
                    </button>
                </div>
            </header>

            <!-- VIEW: OVERVIEW -->
            <div v-if="currentView === 'overview'" class="flex-1 p-8 flex flex-col overflow-hidden gap-6">
                <!-- Stats Row -->
                <div class="grid grid-cols-4 gap-6 shrink-0">
                    <div class="bg-surface p-5 rounded-xl border border-border shadow-sm flex flex-col justify-between">
                        <div class="flex items-center gap-2 text-primary-light text-sm font-medium"><FileText :size="16" /> Processed</div>
                        <div class="text-2xl font-bold tracking-tight">{{ stats.scanned.toLocaleString() }}</div>
                    </div>
                    <div class="bg-surface p-5 rounded-xl border border-border shadow-sm flex flex-col justify-between">
                         <div class="flex items-center gap-2 text-primary-light text-sm font-medium"><AlertTriangle :size="16" /> Risks</div>
                        <div class="text-2xl font-bold tracking-tight text-rose-600">{{ stats.blocked }}</div>
                    </div>
                    <div class="bg-surface p-5 rounded-xl border border-border shadow-sm flex flex-col justify-between col-span-2 relative overflow-hidden group">
                         <div class="flex items-center gap-2 text-primary-light text-sm font-medium relative z-10"><ShieldCheck :size="16" /> Est. Liability</div>
                        <div class="text-2xl font-bold tracking-tight text-primary relative z-10">${{ stats.liability.toLocaleString() }}</div>
                        <div class="absolute right-[-10px] bottom-[-10px] opacity-10"><BarChart :size="80" /></div>
                    </div>
                </div>

                <!-- File Queue List -->
                <div class="flex-1 bg-surface rounded-2xl border border-border flex flex-col overflow-hidden shadow-sm relative">
                    <div class="p-4 border-b border-border bg-gray-50/50 flex justify-between items-center">
                        <div class="flex items-center gap-2">
                             <h3 class="font-semibold text-sm">File Queue</h3>
                             <span v-if="fileQueue.length > 0" class="px-2 py-0.5 rounded-full bg-gray-200 text-xs font-bold">{{ fileQueue.length }}</span>
                        </div>
                    </div>
                    
                    <div v-if="fileQueue.length === 0" class="flex-1 flex flex-col items-center justify-center text-primary-light p-8">
                         <div class="w-16 h-16 rounded-full bg-muted flex items-center justify-center mb-4">
                             <UploadCloud :size="24" />
                         </div>
                         <p class="font-medium">No files in queue</p>
                         <p class="text-sm opacity-70">Click 'Add Files' or drag here</p>
                    </div>

                    <div v-else class="flex-1 flex flex-col overflow-hidden">
                        <!-- Selection Toolbar -->
                        <div class="bg-muted/30 border-b border-border px-4 py-2 flex items-center justify-between">
                            <div class="flex items-center gap-4">
                                <label class="flex items-center gap-2 cursor-pointer">
                                    <input type="checkbox" :checked="selectAll" @change="toggleSelectAll" class="w-4 h-4 rounded border-gray-300" />
                                    <span class="text-sm font-medium">Select All</span>
                                </label>
                                <span v-if="selectedCount > 0" class="text-sm text-primary-light">
                                    {{ selectedCount }} of {{ fileQueue.length }} selected
                                </span>
                            </div>
                            <div class="flex items-center gap-2">
                                <button 
                                    v-if="cleanFilesCount > 0"
                                    @click="clearAllClean"
                                    class="px-3 py-1.5 bg-emerald-600 text-white text-xs font-medium rounded-lg hover:bg-emerald-700 flex items-center gap-1"
                                >
                                    <CheckCircle :size="14" /> Clear Clean ({{ cleanFilesCount }})
                                </button>
                                <button 
                                    v-if="selectedRiskyFiles.length > 0"
                                    @click="bulkSanitizeSelected"
                                    class="px-3 py-1.5 bg-violet-600 text-white text-xs font-medium rounded-lg hover:bg-violet-700 flex items-center gap-1"
                                >
                                    <Sparkles :size="14" /> Sanitize Selected ({{ selectedRiskyFiles.length }})
                                </button>
                                <n-select 
                                    v-model:value="pageSize" 
                                    :options="[
                                        { label: '10 per page', value: 10 },
                                        { label: '25 per page', value: 25 },
                                        { label: '50 per page', value: 50 },
                                        { label: '100 per page', value: 100 }
                                    ]"
                                    size="small"
                                    style="width: 120px"
                                />
                            </div>
                        </div>

                        <!-- File Table -->
                        <div class="flex-1 overflow-y-auto">
                            <table class="w-full text-left text-sm">
                                <thead class="bg-muted/30 sticky top-0 backdrop-blur-sm z-10">
                                    <tr class="border-b border-border">
                                        <th class="px-4 py-3 w-10"></th>
                                        <th class="px-4 py-3 font-medium text-primary-light">File</th>
                                        <th class="px-4 py-3 font-medium text-primary-light">Status</th>
                                        <th class="px-4 py-3 font-medium text-primary-light">Risk Score</th>
                                        <th class="px-4 py-3 font-medium text-primary-light text-right">Action</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr v-for="(file, i) in paginatedFiles" :key="i" 
                                        class="border-b border-border/50 hover:bg-muted/20 group"
                                        :class="{ 'bg-blue-50/50': file.selected }">
                                        <td class="px-4 py-3">
                                            <input 
                                                v-if="file.status === 'pending' || file.status === 'risk'"
                                                type="checkbox" 
                                                :checked="file.selected" 
                                                @change="toggleFileSelection(file)" 
                                                class="w-4 h-4 rounded border-gray-300" 
                                            />
                                            <CheckCircle v-else-if="file.status === 'clean' || file.status === 'sanitized'" :size="16" class="text-emerald-500" />
                                        </td>
                                        <td class="px-4 py-3 font-medium truncate max-w-[200px]">{{ file.path.split(/[\\/]/).pop() }}</td>
                                        <td class="px-4 py-3">
                                        
                                        <div v-if="file.status === 'pending'" class="flex items-center gap-2 text-primary-light"><div class="w-2 h-2 rounded-full bg-gray-300"></div> Pending</div>
                                        <div v-else-if="file.status === 'scanning'" class="flex items-center gap-2 text-blue-600"><Activity :size="14" class="animate-pulse"/> Scanning</div>
                                        <div v-else-if="file.status === 'clean'" class="flex items-center gap-2 text-emerald-600"><CheckCircle :size="14"/> Clean</div>
                                        
                                        <!-- Tooltip for Risks -->
                                        <n-tooltip trigger="hover" v-else-if="file.status === 'risk'">
                                            <template #trigger>
                                                <div class="flex items-center gap-2 text-rose-600 bg-rose-50 px-2 py-1 rounded-full w-fit cursor-help">
                                                    <AlertTriangle :size="14"/> Risk Found <HelpCircle :size="12" class="opacity-50"/>
                                                </div>
                                            </template>
                                            <div class="text-xs max-w-[300px]">
                                                <div class="font-bold mb-1 border-b border-white/20 pb-1">Risk Factors Detected:</div>
                                                <ul class="list-disc pl-4 space-y-0.5" v-if="file.findings && file.findings.length">
                                                    <li v-for="finding in file.findings.slice(0, 5)">{{ finding }}</li>
                                                    <li v-if="file.findings.length > 5" class="italic opacity-80">+{{ file.findings.length - 5 }} more</li>
                                                </ul>
                                                <div v-else class="italic opacity-80">Sensitive patterns found (PHI/PII).</div>
                                                <div class="mt-2 text-[10px] opacity-70 border-t border-white/20 pt-1">
                                                    Sanitize this file to remove these violations.
                                                </div>
                                            </div>
                                        </n-tooltip>

                                        <div v-else-if="file.status === 'sanitized'" class="flex items-center gap-2 text-violet-600 bg-violet-50 px-2 py-1 rounded-full w-fit"><Sparkles :size="14"/> Sanitized</div>
                                        <div v-else-if="file.status === 'error'" class="flex items-center gap-2 text-red-500">Error</div>
                                    </td>
                                    <td class="px-6 py-3 font-mono">
                                        <div class="w-full max-w-[100px] h-1.5 bg-gray-100 rounded-full overflow-hidden" v-if="['risk','clean','sanitized'].includes(file.status)">
                                            <div class="h-full rounded-full" 
                                                :class="file.riskScore > 50 ? 'bg-rose-500' : (file.riskScore > 0 ? 'bg-amber-500' : 'bg-emerald-500')" 
                                                :style="{width: Math.max(5, file.riskScore) + '%'}"
                                            ></div>
                                        </div>
                                        <span v-else>-</span>
                                    </td>
                                    <td class="px-6 py-3 text-right">
                                        <button v-if="file.status === 'risk'" @click="sanitizeSingle(file)" class="p-1.5 rounded-md hover:bg-violet-100 text-violet-600 transition-colors mr-2" title="Sanitize">
                                            <Sparkles :size="16" />
                                        </button>
                                        <button @click="removeFile(i)" class="p-1.5 rounded-md hover:bg-rose-100 text-rose-600 transition-colors opacity-0 group-hover:opacity-100">
                                            <Trash2 :size="16" />
                                        </button>
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                    </div>

                    <!-- Pagination Footer -->
                    <div v-if="totalPages > 1" class="bg-muted/30 border-t border-border px-4 py-2 flex items-center justify-between">
                        <div class="text-xs text-primary-light">
                            Page {{ currentPage }} of {{ totalPages }} ({{ fileQueue.length }} files total)
                        </div>
                        <div class="flex items-center gap-1">
                            <button 
                                @click="currentPage = 1" 
                                :disabled="currentPage === 1"
                                class="px-2 py-1 text-xs rounded hover:bg-white disabled:opacity-30 disabled:cursor-not-allowed"
                            >First</button>
                            <button 
                                @click="currentPage = Math.max(1, currentPage - 1)" 
                                :disabled="currentPage === 1"
                                class="px-2 py-1 text-xs rounded hover:bg-white disabled:opacity-30 disabled:cursor-not-allowed"
                            >&lt; Prev</button>
                            <span class="px-3 py-1 text-xs font-medium bg-white rounded border border-border">{{ currentPage }}</span>
                            <button 
                                @click="currentPage = Math.min(totalPages, currentPage + 1)" 
                                :disabled="currentPage === totalPages"
                                class="px-2 py-1 text-xs rounded hover:bg-white disabled:opacity-30 disabled:cursor-not-allowed"
                            >Next &gt;</button>
                            <button 
                                @click="currentPage = totalPages" 
                                :disabled="currentPage === totalPages"
                                class="px-2 py-1 text-xs rounded hover:bg-white disabled:opacity-30 disabled:cursor-not-allowed"
                            >Last</button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
            <!-- VIEW: ANALYTICS -->
            <div v-if="currentView === 'analytics'" class="flex-1 p-8 overflow-y-auto">
                <!-- Empty State if no scans -->
                <div v-if="stats.scanned === 0" class="flex flex-col items-center justify-center h-full text-center">
                    <PieChart :size="64" class="text-primary-light mb-4" />
                    <h3 class="text-xl font-semibold mb-2">No Analytics Data Yet</h3>
                    <p class="text-primary-light mb-6">Complete a scan to see risk distribution and trends</p>
                    <button @click="currentView = 'overview'" class="px-6 py-3 bg-primary text-white rounded-lg hover:bg-primary/90">
                        Start Scanning
                    </button>
                </div>
                
                <!-- Real Data - Show when scans have been performed -->
                <div v-else class="space-y-6">
                    <div class="grid grid-cols-2 gap-6">
                        <!-- Stats Summary -->
                        <div class="bg-surface p-6 rounded-2xl border border-border shadow-sm">
                            <h3 class="font-semibold mb-4 flex items-center gap-2">
                                <BarChart :size="18"/> Scan Summary
                            </h3>
                            <div class="space-y-3">
                                <div class="flex justify-between items-center">
                                    <span class="text-sm text-primary-light">Total Files Scanned</span>
                                    <span class="font-bold text-lg">{{ stats.scanned }}</span>
                                </div>
                                <div class="flex justify-between items-center">
                                    <span class="text-sm text-primary-light">Risks Found</span>
                                    <span class="font-bold text-lg text-rose-600">{{ stats.blocked }}</span>
                                </div>
                                <div class="flex justify-between items-center">
                                    <span class="text-sm text-primary-light">Est. Liability</span>
                                    <span class="font-bold text-lg text-amber-600">${{ stats.liability.toLocaleString() }}</span>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Risk Status -->
                        <div class="bg-surface p-6 rounded-2xl border border-border shadow-sm">
                            <h3 class="font-semibold mb-4 flex items-center gap-2">
                                <TrendingUp :size="18"/> Current Status
                            </h3>
                            <div class="text-center py-4">
                                <div :class="stats.blocked === 0 ? 'text-emerald-600' : 'text-rose-600'" class="text-4xl font-bold mb-2">
                                    {{ stats.blocked === 0 ? '✓' : '!' }}
                                </div>
                                <div class="font-semibold">
                                    {{ stats.blocked === 0 ? 'All Clear' : stats.blocked + ' Risk' + (stats.blocked > 1 ? 's' : '') + ' Detected' }}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- VIEW: LIVE AUDIT -->
            <div v-if="currentView === 'audit'" class="flex-1 p-8 flex flex-col h-full overflow-hidden">
                <div class="mb-4 flex gap-4">
                    <div class="flex-1 bg-surface border border-border rounded-lg px-4 py-2 flex items-center gap-2">
                        <Search :size="16" class="text-primary-light" />
                        <input v-model="auditSearch" type="text" placeholder="Search audit logs..." class="bg-transparent outline-none w-full text-sm" />
                    </div>
                    <button @click="logs = []" class="px-4 py-2 bg-surface border border-border rounded-lg text-sm hover:bg-muted text-primary">Clear Logs</button>
                </div>
                <div class="flex-1 bg-surface border border-border rounded-2xl overflow-hidden">
                    <table class="w-full">
                        <thead class="bg-muted border-b border-border">
                            <tr>
                                <th class="px-4 py-3 text-left text-xs font-semibold text-primary-light uppercase">Time</th>
                                <th class="px-4 py-3 text-left text-xs font-semibold text-primary-light uppercase">Event</th>
                                <th class="px-4 py-3 text-left text-xs font-semibold text-primary-light uppercase">Type</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-border">
                            <tr v-for="(log, i) in filteredLogs" :key="i" class="hover:bg-muted/30 transition-colors">
                                <td class="px-4 py-3 text-sm font-mono text-primary-light">{{ log.time }}</td>
                                <td class="px-4 py-3 text-sm">{{ log.msg }}</td>
                                <td class="px-4 py-3">
                                    <span class="inline-block px-2 py-0.5 rounded-full text-xs font-medium"
                                          :class="{
                                              'bg-emerald-100 text-emerald-700': log.type === 'success',
                                              'bg-rose-100 text-rose-700': log.type === 'error',
                                              'bg-amber-100 text-amber-700': log.type === 'warning',
                                              'bg-blue-100 text-blue-700': log.type === 'info'
                                          }">
                                        {{ log.type }}
                                    </span>
                                </td>
                            </tr>
                            <tr v-if="filteredLogs.length === 0">
                                <td colspan="3" class="px-4 py-12 text-center text-primary-light text-sm">
                                    No audit logs yet. Start scanning to see activity.
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>
            </div>

            <!-- VIEW: SETTINGS -->
            <div v-if="currentView === 'settings'" class="flex-1 p-8 overflow-y-auto max-w-4xl">
                <!-- General Settings -->
                <div class="bg-surface rounded-2xl border border-border shadow-sm overflow-hidden mb-6">
                    <div class="p-6 border-b border-border">
                        <h3 class="text-lg font-semibold flex items-center gap-2"><Settings :size="20"/> General Settings</h3>
                        <p class="text-sm text-primary-light mt-1">Configure scan parameters and application behavior.</p>
                    <!-- User Profile -->
                <div class="p-4 border-t border-border">
                    <div class="flex items-center gap-3 mb-4">
                        <div class="w-12 h-12 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center text-white font-bold text-lg">
                            {{ currentUser.charAt(0).toUpperCase() }}
                        </div>
                        <div>
                            <span class="text-sm font-medium">{{ currentUser }}</span>
                            <span class="text-xs text-primary-light block">Open Source Edition</span>
                        </div>
                    </div>
                </div>
                    </div>
                    <div class="divide-y divide-border/50">
                        <div class="p-6 flex items-center justify-between">
                            <div>
                                <div class="font-medium">Deep Recursion</div>
                                <div class="text-xs text-primary-light">Scan subdirectories to unlimited depth.</div>
                            </div>
                            <n-switch v-model:value="settings.deepScan" />
                        </div>
                        <div class="p-6 flex items-center justify-between">
                            <div>
                                <div class="font-medium">Notification Sounds</div>
                                <div class="text-xs text-primary-light">Play sound on critical risk detection.</div>
                            </div>
                            <n-switch v-model:value="settings.notificationSound" />
                        </div>
                    </div>
                </div>

                    <!-- Auto-Scan Schedule -->
                    <div class="bg-surface border border-border rounded-2xl overflow-hidden">
                        <div class="p-6 border-b border-border">
                            <h3 class="text-lg font-semibold flex items-center gap-2">
                                <Activity :size="20" class="text-blue-600"/> 
                                Auto-Scan Schedule
                            </h3>
                            <p class="text-sm text-primary-light mt-1">Automatically scan selected folders at regular intervals.</p>
                        </div>
                        
                        <div class="divide-y divide-border/50">
                            <!-- Enable Toggle -->
                            <div class="p-6 flex items-center justify-between">
                                <div>
                                    <div class="font-medium">Enable Scheduled Scans</div>
                                    <div class="text-xs text-primary-light">Background scanning will run automatically.</div>
                                </div>
                                <n-switch v-model:value="scheduleConfig.schedule_enabled" @update:value="saveScheduleConfig" />
                            </div>

                            <!-- Interval Configuration -->
                            <div class="p-6">
                                <div class="font-medium mb-3">Scan Frequency</div>
                                <div class="grid grid-cols-2 gap-4">
                                    <div>
                                        <label class="text-xs text-primary-light block mb-2">Every</label>
                                        <n-input-number 
                                            v-model:value="scheduleConfig.interval_value" 
                                            :min="1" 
                                            :max="100"
                                            class="w-full"
                                            @blur="saveScheduleConfig"
                                        />
                                    </div>
                                    <div>
                                        <label class="text-xs text-primary-light block mb-2">Unit</label>
                                        <n-select 
                                            v-model:value="scheduleConfig.interval_unit" 
                                            :options="[
                                                { label: 'Hours', value: 'hours' },
                                                { label: 'Days', value: 'days' },
                                                { label: 'Weeks', value: 'weeks' },
                                                { label: 'Months', value: 'months' }
                                            ]"
                                            @update:value="saveScheduleConfig"
                                        />
                                    </div>
                                </div>
                                <div class="mt-2 text-xs text-primary-light">
                                    Scans will run every {{ scheduleConfig.interval_value }} {{ scheduleConfig.interval_unit }}
                                </div>
                            </div>

                            <!-- Time of Day -->
                            <div class="p-6">
                                <div class="font-medium mb-3">Time of Day</div>
                                <div class="grid grid-cols-2 gap-4">
                                    <div>
                                        <label class="text-xs text-primary-light block mb-2">Run At</label>
                                        <input 
                                            type="time" 
                                            v-model="scheduleConfig.time_of_day"
                                            @change="saveScheduleConfig"
                                            class="w-full px-3 py-2 border border-border rounded-lg bg-surface text-sm"
                                        />
                                    </div>
                                    <div>
                                        <label class="text-xs text-primary-light block mb-2">Timezone</label>
                                        <n-select 
                                            v-model:value="scheduleConfig.timezone"
                                            :options="[
                                                { label: 'Eastern (ET)', value: 'America/New_York' },
                                                { label: 'Central (CT)', value: 'America/Chicago' },
                                                { label: 'Mountain (MT)', value: 'America/Denver' },
                                                { label: 'Pacific (PT)', value: 'America/Los_Angeles' },
                                                { label: 'UTC', value: 'UTC' }
                                            ]"
                                            @update:value="saveScheduleConfig"
                                        />
                                    </div>
                                </div>
                                <div class="mt-2 text-xs text-primary-light">
                                    Scheduled scans will start at {{ scheduleConfig.time_of_day }} {{ scheduleConfig.timezone }}
                                </div>
                            </div>

                            <!-- Manual Trigger -->
                            <div class="p-6 flex items-center justify-between">
                                <div>
                                    <div class="font-medium">Quick Scan</div>
                                    <div class="text-xs text-primary-light">
                                        {{ isScheduledScanning ? 'Scanning in progress...' : 'Run immediately, regardless of schedule' }}
                                    </div>
                                </div>
                                <button 
                                    @click="triggerScanNow"
                                    :disabled="isScheduledScanning"
                                    class="px-4 py-2 text-white text-sm rounded-lg transition-all font-medium flex items-center gap-2"
                                    :class="isScheduledScanning ? 'bg-blue-500 cursor-not-allowed animate-pulse' : 'bg-blue-600 hover:bg-blue-700'"
                                >
                                    <template v-if="isScheduledScanning">
                                        <span class="flex items-center gap-1">
                                            <span class="w-1.5 h-1.5 bg-white rounded-full animate-bounce" style="animation-delay: 0s"></span>
                                            <span class="w-1.5 h-1.5 bg-white rounded-full animate-bounce" style="animation-delay: 0.1s"></span>
                                            <span class="w-1.5 h-1.5 bg-white rounded-full animate-bounce" style="animation-delay: 0.2s"></span>
                                        </span>
                                        Scanning
                                    </template>
                                    <template v-else>
                                        <Activity :size="16" /> Scan Now
                                    </template>
                                </button>
                            </div>

                            <!-- Monitored Folders -->
                        <div class="p-6">
                            <div class="flex justify-between items-center mb-4">
                                <div class="font-medium">Monitored Folders</div>
                                <button @click="addSchedulePath" class="px-3 py-1.5 rounded-md bg-blue-600 text-white text-xs font-bold hover:bg-blue-500 transition-colors">
                                    + Add Folder
                                </button>
                            </div>
                            
                            <div v-if="scheduleConfig.scan_paths.length === 0" class="text-center py-8 text-primary-light text-sm">
                                No folders added yet. Click "Add Folder" to start.
                            </div>
                            
                            <div v-else class="space-y-2">
                                <div v-for="(path, i) in scheduleConfig.scan_paths" :key="i" 
                                     class="flex items-center justify-between p-3 bg-muted rounded-lg group">
                                    <div class="flex items-center gap-2 flex-1 min-w-0">
                                        <FolderSearch :size="16" class="text-blue-600 shrink-0"/>
                                        <span class="text-sm truncate">{{ path }}</span>
                                    </div>
                                    <button @click="removeSchedulePath(path)" 
                                            class="p-1.5 rounded-md hover:bg-rose-100 text-rose-600 opacity-0 group-hover:opacity-100 transition-opacity">
                                        <Trash2 :size="14" />
                                    </button>
                                </div>
                            </div>
                        </div>

                        <!-- Audit History -->
                        <div class="p-6">
                            <div class="font-medium mb-4 flex items-center gap-2">
                                <FileText :size="16"/>
                                Audit History
                                <span class="text-xs bg-gray-200 px-2 py-0.5 rounded-full">Last {{scheduleConfig.audit_history.length}}</span>
                            </div>
                            
                            <div v-if="scheduleConfig.audit_history.length === 0" class="text-center py-4 text-primary-light text-sm">
                                No audit history yet.
                            </div>
                            
                            <div v-else class="space-y-2 max-h-64 overflow-y-auto">
                                <div v-for="(entry, i) in scheduleConfig.audit_history.slice(0, 10)" :key="i"
                                     class="flex items-center justify-between p-3 bg-muted/50 rounded-lg text-sm">
                                    <div class="flex items-center gap-3 flex-1">
                                        <div class="w-2 h-2 rounded-full" :class="entry.status === 'PASSED' ? 'bg-emerald-500' : 'bg-rose-500'"></div>
                                        <div class="font-mono text-xs text-primary-light">{{ new Date(entry.timestamp).toLocaleString() }}</div>
                                        <div class="text-xs">{{ entry.total_files }} files</div>
                                    </div>
                                    <div :class="entry.status === 'PASSED' ? 'text-emerald-600 font-bold' : 'text-rose-600 font-bold'" class="text-xs">
                                        {{ entry.status }}
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

        </main>
        
        <!-- Overlay -->
        <div v-if="showScanOverlay" class="fixed inset-0 z-[100] bg-black/80 backdrop-blur-md flex flex-col items-center justify-center text-white" style="--wails-draggable:drag">
             <div class="w-24 h-24 mb-8 relative">
                <div class="absolute inset-0 border-4 border-t-emerald-500 border-r-transparent border-b-transparent border-l-transparent rounded-full animate-spin"></div>
                <div class="absolute inset-2 border-4 border-t-transparent border-r-blue-500 border-b-transparent border-l-transparent rounded-full animate-spin" style="animation-direction: reverse; animation-duration: 1.5s"></div>
                <div class="absolute inset-0 flex items-center justify-center"><ShieldCheck :size="32" class="text-white animate-pulse" /></div>
            </div>
            <h2 class="text-2xl font-bold tracking-tight mb-2">Guard Engine Active</h2>
             <button @click="stopScan" class="mt-8 px-6 py-2 rounded-full bg-white/10 hover:bg-white/20 border border-white/20 text-sm font-medium transition-colors" style="--wails-draggable:no-drag">Cancel</button>
        </div>

      </div>
    </n-message-provider>
  </n-config-provider>
</template>

<style>
/* Custom Scrollbar */
::-webkit-scrollbar { width: 6px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background-color: #e2e8f0; border-radius: 20px; }
::-webkit-scrollbar-thumb:hover { background-color: #cbd5e1; }

/* Indeterminate Progress Bar Animation */
@keyframes progress-indeterminate {
  0% { transform: translateX(-100%); width: 40%; }
  50% { transform: translateX(100%); width: 60%; }
  100% { transform: translateX(300%); width: 40%; }
}
.animate-progress-indeterminate {
  animation: progress-indeterminate 1.5s ease-in-out infinite;
}

/* Loader dot animation */
@keyframes loader-dots {
  0%, 80%, 100% { opacity: 0.3; }
  40% { opacity: 1; }
}
</style>
