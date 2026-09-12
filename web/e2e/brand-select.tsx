import React,{useState} from 'react';
import {createRoot} from 'react-dom/client';
import BrandSelect from '../src/components/AgentWorkspace/BrandSelect';
import {ProviderIcon} from '../src/components/AgentWorkspace/ProviderIcon';
import '../src/styles/index.css';
function Test(){const [value,setValue]=useState('openai');return <main style={{padding:40}}><BrandSelect label="Model" value={value} onChange={setValue} options={['openai','anthropic','gemini','deepseek','kimi','zhipu','custom'].map(value=>({value,label:value,icon:<ProviderIcon provider={value}/>}))}/><p id="selected">{value}</p></main>;}
createRoot(document.getElementById('root')!).render(<Test/>);
