import React, { useState, useEffect } from 'react';
import { X, Play, Code, Info, Table as TableIcon } from 'lucide-react';

interface TableExplorerProps {
  modelName: string;
  onClose: () => void;
}

interface ModelDetails {
  Name: string;
  SQL: string;
  Dependencies: string[];
  Config: Record<string, string>;
  Description: string;
}

export function TableExplorer({ modelName, onClose }: TableExplorerProps) {
  const [activeTab, setActiveTab] = useState<'details' | 'sql' | 'preview'>('details');
  const [details, setDetails] = useState<ModelDetails | null>(null);
  const [previewData, setPreviewData] = useState<any[] | null>(null);
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    // Reset state when model changes
    setDetails(null);
    setPreviewData(null);
    setActiveTab('details');
    setError('');

    // Fetch details
    fetch(`/api/models/${modelName}`)
      .then(res => {
        if (!res.ok) throw new Error("Failed to fetch model details");
        return res.json();
      })
      .then(data => setDetails(data))
      .catch(err => setError(err.message));
  }, [modelName]);

  const loadPreview = () => {
    setLoadingPreview(true);
    fetch(`/api/models/${modelName}/preview`)
      .then(res => {
        if (!res.ok) throw new Error("Failed to fetch preview data");
        return res.json();
      })
      .then(data => {
        setPreviewData(data.rows || []);
        setActiveTab('preview');
      })
      .catch(err => setError(err.message))
      .finally(() => setLoadingPreview(false));
  };

  if (!details && !error) return <div className="p-4">Loading model details...</div>;

  return (
    <div className="flex flex-col h-full bg-white border-l border-gray-200 shadow-xl overflow-hidden w-96 md:w-[500px]">
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-gray-200 bg-gray-50">
        <div className="flex items-center space-x-2">
          <TableIcon className="w-5 h-5 text-indigo-600" />
          <h2 className="text-lg font-bold text-gray-800">{modelName}</h2>
        </div>
        <button onClick={onClose} className="p-1 text-gray-500 hover:text-gray-800 rounded-full hover:bg-gray-200 transition-colors">
          <X className="w-5 h-5" />
        </button>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-gray-200 bg-white">
        <button
          className={`flex-1 flex items-center justify-center space-x-2 py-3 text-sm font-medium border-b-2 transition-colors ${activeTab === 'details' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
          onClick={() => setActiveTab('details')}
        >
          <Info className="w-4 h-4" />
          <span>Details</span>
        </button>
        <button
          className={`flex-1 flex items-center justify-center space-x-2 py-3 text-sm font-medium border-b-2 transition-colors ${activeTab === 'sql' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
          onClick={() => setActiveTab('sql')}
        >
          <Code className="w-4 h-4" />
          <span>SQL Code</span>
        </button>
        <button
          className={`flex-1 flex items-center justify-center space-x-2 py-3 text-sm font-medium border-b-2 transition-colors ${activeTab === 'preview' ? 'border-indigo-600 text-indigo-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}
          onClick={() => {
            setActiveTab('preview');
            if (!previewData && !loadingPreview) loadPreview();
          }}
        >
          <Play className="w-4 h-4" />
          <span>Preview</span>
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4 bg-gray-50">
        {error && (
          <div className="mb-4 p-3 bg-red-100 text-red-700 rounded-md text-sm border border-red-200">
            {error}
          </div>
        )}

        {activeTab === 'details' && details && (
          <div className="space-y-4">
            <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200">
              <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-3">Configuration</h3>
              <div className="grid grid-cols-2 gap-y-3">
                {Object.entries(details.Config || {}).map(([k, v]) => (
                  <React.Fragment key={k}>
                    <span className="text-sm font-medium text-gray-700 capitalize">{k.replace('_', ' ')}</span>
                    <span className="text-sm text-gray-900 font-mono bg-gray-100 px-2 py-0.5 rounded w-fit">{v as string}</span>
                  </React.Fragment>
                ))}
              </div>
            </div>

            <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-200">
              <h3 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-3">Dependencies</h3>
              {details.Dependencies && details.Dependencies.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {details.Dependencies.map(dep => (
                    <span key={dep} className="text-xs font-medium bg-blue-100 text-blue-800 px-2.5 py-1 rounded-full border border-blue-200">
                      {dep}
                    </span>
                  ))}
                </div>
              ) : (
                <p className="text-sm text-gray-500 italic">No upstream dependencies</p>
              )}
            </div>
          </div>
        )}

        {activeTab === 'sql' && details && (
          <div className="h-full bg-slate-900 rounded-lg p-4 shadow-inner overflow-auto">
            <pre className="text-sm text-green-400 font-mono leading-relaxed">
              <code>{details.SQL}</code>
            </pre>
          </div>
        )}

        {activeTab === 'preview' && (
          <div className="h-full flex flex-col">
            {!previewData ? (
              <div className="flex-1 flex flex-col items-center justify-center space-y-4">
                <p className="text-sm text-gray-500 text-center max-w-xs">Run a live query against your isolated environment to preview this model's data.</p>
                <button 
                  onClick={loadPreview}
                  disabled={loadingPreview}
                  className="flex items-center space-x-2 bg-indigo-600 hover:bg-indigo-700 text-white px-4 py-2 rounded-md font-medium shadow-sm transition-all disabled:opacity-70"
                >
                  {loadingPreview ? <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div> : <Play className="w-4 h-4" />}
                  <span>{loadingPreview ? 'Running Query...' : 'Run Preview Query'}</span>
                </button>
              </div>
            ) : previewData.length === 0 ? (
              <div className="flex-1 flex items-center justify-center">
                <p className="text-sm text-gray-500 italic">Query returned 0 rows.</p>
              </div>
            ) : (
              <div className="bg-white rounded-lg border border-gray-200 shadow-sm overflow-auto">
                <table className="min-w-full divide-y divide-gray-200 text-sm">
                  <thead className="bg-gray-50 sticky top-0">
                    <tr>
                      {Object.keys(previewData[0]).map(key => (
                        <th key={key} className="px-4 py-3 text-left text-xs font-semibold text-gray-600 uppercase tracking-wider whitespace-nowrap">
                          {key}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody className="bg-white divide-y divide-gray-200">
                    {previewData.map((row, i) => (
                      <tr key={i} className="hover:bg-indigo-50/50 transition-colors">
                        {Object.values(row).map((val: any, j) => (
                          <td key={j} className="px-4 py-2.5 whitespace-nowrap text-gray-700 font-mono text-xs">
                            {val !== null ? String(val) : <span className="text-gray-400 italic">null</span>}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
