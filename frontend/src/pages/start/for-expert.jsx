import { useCreateTemplate } from "../../apis/template"
import { useStartStore } from '../../store/useStartStore'
import { Button } from "@/components/ui/button"
import { PiSpinnerGap } from "react-icons/pi"
import { IoArrowForwardSharp } from "react-icons/io5"
import { useState } from "react"
import AceEditor from "react-ace";
import "ace-builds/src-noconflict/mode-json";
import "ace-builds/src-noconflict/theme-github";
import "ace-builds/src-noconflict/ext-language_tools";
import { useNavigate } from "react-router-dom";
export function ForExpert() {
    const { trigger: createTemplate, loading: createTemplateLoading, error: createTemplateError } = useCreateTemplate()
    const { setStep } = useStartStore()
    const [data, setData] = useState('')
    const navigate = useNavigate()
    return (
        <>
            <div className='mt-8 text-xl'>通过配置创建</div>
            <div className='opacity-70 text-sm mt-2'>不清楚配置？返回通过模板部署。</div>
            <div className='mt-4 border rounded-lg overflow-hidden'>

            <AceEditor
                style={{ width: '100%' }}
                mode="json"
                theme="github"
                onChange={(value) => setData(value)}
                name="UNIQUE_ID_OF_DIV"
                editorProps={{ $blockScrolling: true }}
            />
            </div>
            <div className='mt-8 flex justify-between'>
                <Button variant='outline' className='cursor-pointer' onClick={() => setStep('select-user-type')}>返回</Button>
                <Button disabled={createTemplateLoading} className='cursor-pointer' onClick={async() => {
                    if (!data || createTemplateLoading) return
                    await createTemplate({ data })
                    navigate('/')
                }}>部署 {createTemplateLoading ? <PiSpinnerGap className='animate-spin' /> : <IoArrowForwardSharp />}</Button>
            </div>
            {createTemplateError && <div className='bg-red-800 text-white p-4 mt-8 rounded-lg'>
                {createTemplateError?.message}
            </div>}
        </>
    )
}